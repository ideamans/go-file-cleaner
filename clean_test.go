package filecleaner

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/go-test/deep"
)

func TestClean(t *testing.T) {
	totalEntries := uint64(14)

	cases := []struct {
		name         string
		fileTimeType FileTimeType
		fileSize     uint64
		blockSize    uint64
		targetSize   uint64
		timeUnit     time.Duration
		concurrently int
		examples     []string
		entries      []string
	}{
		{
			name:         "mtime-remove1-concurrently1",
			fileTimeType: ModTime,
			fileSize:     2048,
			blockSize:    4096,
			targetSize:   4096*totalEntries - 4000,
			timeUnit:     time.Hour,
			concurrently: 1,
			examples:     []string{"2/2/1"},
			entries:      []string{".", "1", "1/1", "1/1/1", "1/1/2", "1/2", "1/2/1", "1/2/2", "2", "2/1", "2/1/1", "2/1/2", "2/2", "2/2/2"},
		},
		{
			name:         "atime-remove1-concurrently1",
			fileTimeType: AccessTime,
			fileSize:     2048,
			blockSize:    4096,
			targetSize:   4096*totalEntries - 4000,
			timeUnit:     time.Hour,
			concurrently: 1,
			examples:     []string{"1/1/1"},
			entries:      []string{".", "1", "1/1", "1/1/2", "1/2", "1/2/1", "1/2/2", "2", "2/1", "2/1/1", "2/1/2", "2/2", "2/2/1", "2/2/2"},
		},
		{
			name:         "mtime-remove2-concurrently1",
			fileTimeType: ModTime,
			fileSize:     2048,
			blockSize:    4096,
			targetSize:   4096*totalEntries - 5000,
			timeUnit:     time.Hour,
			concurrently: 1,
			examples:     []string{"2/2/1", "2/2/2"},
			entries:      []string{".", "1", "1/1", "1/1/1", "1/1/2", "1/2", "1/2/1", "1/2/2", "2", "2/1", "2/1/1", "2/1/2"},
		},
		{
			name:         "atime-remove2-concurrently1",
			fileTimeType: AccessTime,
			fileSize:     2048,
			blockSize:    4096,
			targetSize:   4096*totalEntries - 5000,
			timeUnit:     time.Hour,
			concurrently: 1,
			examples:     []string{"1/1/1", "1/1/2"},
			entries:      []string{".", "1", "1/2", "1/2/1", "1/2/2", "2", "2/1", "2/1/1", "2/1/2", "2/2", "2/2/1", "2/2/2"},
		},
		{
			name:         "mtime-remove1-concurrently2",
			fileTimeType: ModTime,
			fileSize:     2048,
			blockSize:    4096,
			targetSize:   4096*totalEntries - 4000,
			timeUnit:     time.Hour,
			concurrently: 1,
			examples:     []string{"2/2/1"},
			entries:      []string{".", "1", "1/1", "1/1/1", "1/1/2", "1/2", "1/2/1", "1/2/2", "2", "2/1", "2/1/1", "2/1/2", "2/2", "2/2/2"},
		},
		{
			name:         "atime-remove1-concurrently2",
			fileTimeType: AccessTime,
			fileSize:     2048,
			blockSize:    4096,
			targetSize:   4096*totalEntries - 4000,
			timeUnit:     time.Hour,
			concurrently: 1,
			examples:     []string{"1/1/1"},
			entries:      []string{".", "1", "1/1", "1/1/2", "1/2", "1/2/1", "1/2/2", "2", "2/1", "2/1/1", "2/1/2", "2/2", "2/2/1", "2/2/2"},
		},
	}

	for _, c := range cases {
		temp, _ := os.MkdirTemp("", "clean_test_"+c.name)
		err := createDummyDirectoryTree(temp, 3, 2, 2, 2048)
		if err != nil {
			t.Fatal(err)
		}

		// 1/1/1 atime 2days ago 1/1/2 atime 1day ago
		// 2/2/1 mtime 2days ago 2/2/2 mtime 1day ago

		now := time.Now()
		twoDaysAgo := now.Add(-48 * time.Hour)
		oneDayAgo := now.Add(-24 * time.Hour)
		os.Chtimes(filepath.Join(temp, "1/1/1"), twoDaysAgo, now)
		os.Chtimes(filepath.Join(temp, "1/1/2"), oneDayAgo, now)
		os.Chtimes(filepath.Join(temp, "2/2/1"), now, twoDaysAgo)
		os.Chtimes(filepath.Join(temp, "2/2/2"), now, oneDayAgo)

		input := &CleaningInput{
			RootPath:     temp,
			BlockSize:    c.blockSize,
			TargetSize:   c.targetSize,
			FileTimeType: c.fileTimeType,
			TimeBin:      c.timeUnit,
			Pattern:      "",
			Examples:     10,
			Concurrency:  c.concurrently,
			DryRun:       false,
		}

		result, err := Clean(input)

		if err != nil {
			t.Fatal(err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		sort.Strings(result.Examples)
		examplesDiff := deep.Equal(result.Examples, c.examples)
		if examplesDiff != nil {
			t.Error(examplesDiff)
		}

		entries := []string{}
		err = filepath.Walk(temp, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			relPath, err := filepath.Rel(temp, path)
			if err != nil {
				return nil
			}
			entries = append(entries, relPath)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

		sort.Strings(entries)
		entriesDiff := deep.Equal(entries, c.entries)
		if entriesDiff != nil {
			t.Error(c.name, entriesDiff)
		}
	}
}
