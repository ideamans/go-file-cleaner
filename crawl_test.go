package filecleaner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/go-test/deep"
)

// ダミーのディレクトリツリー作成関数
func createDummyDirectoryTree(dirPath string, depth, children, files int, size uint64) error {
	return createRecursiveDirectories(dirPath, 1, depth, children, files, size)
}

// 再帰的なダミーディレクトリ構造作成
func createRecursiveDirectories(currentPath string, currentLevel, maxDepth, children, files int, size uint64) error {
	// 末端ではファイルを作成
	if currentLevel == maxDepth {
		for i := 1; i <= files; i++ {
			filePath := filepath.Join(currentPath, fmt.Sprintf("%d", i))
			if err := createNullFile(filePath, size); err != nil {
				return fmt.Errorf("failed to create file at %s: %w", filePath, err)
			}
		}
		return nil
	}

	// 中間層では再帰的にディレクトリを作成
	for i := 1; i <= children; i++ {
		dirPath := filepath.Join(currentPath, fmt.Sprintf("%d", i))
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
		}
		if err := createRecursiveDirectories(dirPath, currentLevel+1, maxDepth, children, files, size); err != nil {
			return err
		}
	}

	return nil
}

// 指定サイズのヌルバイトファイルを作成
func createNullFile(filePath string, size uint64) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	nullBytes := make([]byte, size)
	if _, err = f.Write(nullBytes); err != nil {
		return err
	}
	return nil
}

// 並列度クローリングのテスト
func TestCrawl(t *testing.T) {
	relPathsWhenFullPattern := []string{
		"1",
		filepath.Join("1", "1"),
		filepath.Join("1", "1", "1"), filepath.Join("1", "1", "2"), filepath.Join("1", "1", "3"),
		filepath.Join("1", "2"),
		filepath.Join("1", "2", "1"), filepath.Join("1", "2", "2"), filepath.Join("1", "2", "3"),
		"2",
		filepath.Join("2", "1"),
		filepath.Join("2", "1", "1"), filepath.Join("2", "1", "2"), filepath.Join("2", "1", "3"),
		filepath.Join("2", "2"),
		filepath.Join("2", "2", "1"), filepath.Join("2", "2", "2"), filepath.Join("2", "2", "3"),
	}
	filesWhenFullPattern := uint64(12)

	pattern := "*/2/**"
	relPathsWhenPatterned := []string{
		filepath.Join("1", "2"),
		filepath.Join("1", "2", "1"),
		filepath.Join("1", "2", "2"),
		filepath.Join("1", "2", "3"),
		filepath.Join("2", "2"),
		filepath.Join("2", "2", "1"),
		filepath.Join("2", "2", "2"),
		filepath.Join("2", "2", "3"),
	}
	filesWhenPatterned := uint64(6)

	cases := []struct {
		name        string
		concurrency uint
		pattern     string
		expected    []string
		files       uint64
	}{
		{"concurrency:1,without-pattern", 1, "", relPathsWhenFullPattern, filesWhenFullPattern},
		{"concurrency:1,with-pattern", 1, pattern, relPathsWhenPatterned, filesWhenPatterned},
		{"concurrency:2,without-pattern", 2, "", relPathsWhenFullPattern, filesWhenFullPattern},
		{"concurrency:2,with-pattern", 2, pattern, relPathsWhenPatterned, filesWhenPatterned},
		{"concurrency:4,without-pattern", 4, "", relPathsWhenFullPattern, filesWhenFullPattern},
		{"concurrency:4,with-pattern", 4, pattern, relPathsWhenPatterned, filesWhenPatterned},
	}

	for _, c := range cases {
		temp, err := os.MkdirTemp("", "crawler_test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(temp)

		fileSize := uint64(10240)
		err = createDummyDirectoryTree(temp, 3, 2, 3, fileSize)
		if err != nil {
			t.Fatal(err)
		}

		var relPaths []string
		var totalSize uint64
		var mu sync.Mutex
		input := CrawlingInput{
			RootPath:    temp,
			Concurrency: c.concurrency,
			Pattern:     c.pattern,
		}

		err = Crawl(&input, func(rootPath, fullPath string, fileInfo os.FileInfo) {
			relPath, err := filepath.Rel(rootPath, fullPath)
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			relPaths = append(relPaths, relPath)
			if !fileInfo.IsDir() {
				totalSize += uint64(fileInfo.Size())
			}
			mu.Unlock()
		}, func(rootPath, fullPath string, err error) {
			t.Errorf("error on %s: %v", fullPath, err)
		})
		if err != nil {
			t.Error(err)
		}

		sort.Strings(relPaths)
		diff := deep.Equal(relPaths, c.expected)
		if len(diff) > 0 {
			t.Error(diff)
		}

		if totalSize != fileSize*c.files {
			t.Errorf("totalSize want %d got %d", fileSize*c.files, totalSize)
		}
	}
}
