package filecleaner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/go-test/deep"
)

// 閾値が未来の時刻にならないことを検証するテスト
// https://github.com/ideamans/go-file-cleaner/issues/X
// 天井丸め(ceiling)だと現在時刻20:05のファイルがビン21:00に入り、
// 閾値が未来になって全ファイルが削除される問題があった
func TestThresholdShouldNotExceedCurrentTime(t *testing.T) {
	temp, _ := os.MkdirTemp("", "threshold_test")
	defer os.RemoveAll(temp)

	// 10個のファイルを作成（各2048バイト、ブロックサイズ4096）
	// 半分は1時間前、半分は現在時刻のタイムスタンプ
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	for i := 0; i < 10; i++ {
		filePath := filepath.Join(temp, fmt.Sprintf("file%d.dat", i))
		f, err := os.Create(filePath)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(make([]byte, 2048))
		f.Close()

		if i < 5 {
			os.Chtimes(filePath, oneHourAgo, oneHourAgo)
		}
		// i >= 5 のファイルは現在時刻のまま
	}

	// ターゲットサイズを非常に小さく設定し、ほぼ全ファイルの削除を要求
	input := &CleaningInput{
		RootPath:     temp,
		BlockSize:    4096,
		TargetSize:   4096, // 1ブロック分だけ残す
		FileTimeType: ModTime,
		TimeBin:      time.Hour,
		Pattern:      "",
		Examples:     10,
		Concurrency:  1,
		DryRun:       true, // 実際には削除しない
	}

	result, err := Clean(input)
	if err != nil {
		t.Fatal(err)
	}

	// 閾値が未来の時刻でないことを確認
	if !result.RemoveBefore.IsZero() && result.RemoveBefore.After(now) {
		t.Errorf("threshold time %s is in the future (now: %s)", result.RemoveBefore.Format(time.RFC3339), now.Format(time.RFC3339))
	}
}

func TestClean(t *testing.T) {
	totalEntries := uint64(14)

	// ヘルパー: スラッシュ区切りのパスをOS区切りに変換
	p := filepath.FromSlash

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
			examples:     []string{p("2/2/1")},
			entries:      []string{".", "1", p("1/1"), p("1/1/1"), p("1/1/2"), p("1/2"), p("1/2/1"), p("1/2/2"), "2", p("2/1"), p("2/1/1"), p("2/1/2"), p("2/2"), p("2/2/2")},
		},
		{
			name:         "atime-remove1-concurrently1",
			fileTimeType: AccessTime,
			fileSize:     2048,
			blockSize:    4096,
			targetSize:   4096*totalEntries - 4000,
			timeUnit:     time.Hour,
			concurrently: 1,
			examples:     []string{p("1/1/1")},
			entries:      []string{".", "1", p("1/1"), p("1/1/2"), p("1/2"), p("1/2/1"), p("1/2/2"), "2", p("2/1"), p("2/1/1"), p("2/1/2"), p("2/2"), p("2/2/1"), p("2/2/2")},
		},
		{
			name:         "mtime-remove2-concurrently1",
			fileTimeType: ModTime,
			fileSize:     2048,
			blockSize:    4096,
			targetSize:   4096*totalEntries - 5000,
			timeUnit:     time.Hour,
			concurrently: 1,
			examples:     []string{p("2/2/1"), p("2/2/2")},
			entries:      []string{".", "1", p("1/1"), p("1/1/1"), p("1/1/2"), p("1/2"), p("1/2/1"), p("1/2/2"), "2", p("2/1"), p("2/1/1"), p("2/1/2")},
		},
		{
			name:         "atime-remove2-concurrently1",
			fileTimeType: AccessTime,
			fileSize:     2048,
			blockSize:    4096,
			targetSize:   4096*totalEntries - 5000,
			timeUnit:     time.Hour,
			concurrently: 1,
			examples:     []string{p("1/1/1"), p("1/1/2")},
			entries:      []string{".", "1", p("1/2"), p("1/2/1"), p("1/2/2"), "2", p("2/1"), p("2/1/1"), p("2/1/2"), p("2/2"), p("2/2/1"), p("2/2/2")},
		},
		{
			name:         "mtime-remove1-concurrently2",
			fileTimeType: ModTime,
			fileSize:     2048,
			blockSize:    4096,
			targetSize:   4096*totalEntries - 4000,
			timeUnit:     time.Hour,
			concurrently: 1,
			examples:     []string{p("2/2/1")},
			entries:      []string{".", "1", p("1/1"), p("1/1/1"), p("1/1/2"), p("1/2"), p("1/2/1"), p("1/2/2"), "2", p("2/1"), p("2/1/1"), p("2/1/2"), p("2/2"), p("2/2/2")},
		},
		{
			name:         "atime-remove1-concurrently2",
			fileTimeType: AccessTime,
			fileSize:     2048,
			blockSize:    4096,
			targetSize:   4096*totalEntries - 4000,
			timeUnit:     time.Hour,
			concurrently: 1,
			examples:     []string{p("1/1/1")},
			entries:      []string{".", "1", p("1/1"), p("1/1/2"), p("1/2"), p("1/2/1"), p("1/2/2"), "2", p("2/1"), p("2/1/1"), p("2/1/2"), p("2/2"), p("2/2/1"), p("2/2/2")},
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
