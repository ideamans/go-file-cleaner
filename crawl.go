package filecleaner

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
)

type CrawlingInput struct {
	RootPath    string
	Concurrency uint
	Pattern     string
}

type EntryHandler func(rootPath, fullPath string, fileInfo os.FileInfo)
type ErrorHandler func(rootPath, fullPath string, err error)

func DefaultErrorHandler(rootPath, fullPath string, err error) {
	log.Printf("failed to handle %s: %v", fullPath, err)
}

// ディレクトリの再帰的な走査を行い、エントリごとにentryHandlerを呼び出す
func Crawl(input *CrawlingInput, entryHandler EntryHandler, errorHandler ErrorHandler) error {
	if input.Concurrency == 1 {
		return crawlSerially(input, entryHandler, errorHandler)
	} else {
		return crawlConcurrently(input, entryHandler, errorHandler)
	}
}

// パターンマッチをする
func matchPattern(pattern, rootPath, fullPath string) (bool, error) {
	relPath, err := filepath.Rel(rootPath, fullPath)
	if err != nil {
		return false, fmt.Errorf("failed to get relative path from %s on %s: %v", fullPath, rootPath, err)
	}
	matched, err := doublestar.Match(pattern, filepath.ToSlash(relPath))
	if err != nil {
		return false, fmt.Errorf("failed to match pattern %s on %s: %v", pattern, relPath, err)
	}
	return matched, nil
}

// エントリを処理する共通関数
func processEntry(rootPath, pattern, dirPath string, entry os.DirEntry, entryHandler EntryHandler, errorHandler ErrorHandler) (string, os.FileInfo, bool) {
	fullPath := filepath.Join(dirPath, entry.Name())

	fileInfo, err := entry.Info()
	if err != nil {
		if errorHandler != nil {
			errorHandler(rootPath, fullPath, err)
		}
		return fullPath, nil, false
	}

	if pattern != "" {
		matched, err := matchPattern(pattern, rootPath, fullPath)
		if err != nil {
			if errorHandler != nil {
				errorHandler(rootPath, fullPath, err)
			}
			return fullPath, fileInfo, false
		}
		if matched {
			entryHandler(rootPath, fullPath, fileInfo)
		}
	} else {
		entryHandler(rootPath, fullPath, fileInfo)
	}

	return fullPath, fileInfo, true
}

// 並列度1のケースにおいて再帰的な走査をする
func crawlRecursive(rootPath string, pattern string, dirPath string, entryHandler EntryHandler, errorHandler ErrorHandler) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fullPath, fileInfo, cont := processEntry(rootPath, pattern, dirPath, entry, entryHandler, errorHandler)
		if !cont {
			continue
		}

		if fileInfo.IsDir() {
			crawlRecursive(rootPath, pattern, fullPath, entryHandler, errorHandler)
		}
	}

	return nil
}

// 並列度1におけるエントリポイント
func crawlSerially(input *CrawlingInput, entryHandler EntryHandler, errorHandler ErrorHandler) error {
	if entryHandler == nil {
		return fmt.Errorf("entryHandler must be specified")
	}

	err := crawlRecursive(input.RootPath, input.Pattern, input.RootPath, entryHandler, errorHandler)
	if err != nil {
		return err
	}
	return nil
}

// 複数の並列度でチャネルを用い再帰的に操作する
func crawlConcurrently(input *CrawlingInput, entryHandler EntryHandler, errorHandler ErrorHandler) error {
	if entryHandler == nil {
		return fmt.Errorf("entryHandler must be specified")
	}

	var dirWg, workerWg sync.WaitGroup
	dirCh := make(chan string, 1000)

	for i := 0; i < int(input.Concurrency); i++ {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()

			for dirPath := range dirCh {
				entries, err := os.ReadDir(dirPath)
				if err != nil {
					dirWg.Done()
					continue
				}

				for _, entry := range entries {
					fullPath, fileInfo, cont := processEntry(input.RootPath, input.Pattern, dirPath, entry, entryHandler, errorHandler)
					if !cont {
						continue
					}

					if fileInfo.IsDir() {
						dirWg.Add(1)
						// デッドロックを回避するためにgoroutine内でdirChに書き込む
						go func() {
							dirCh <- fullPath
						}()
					}
				}

				dirWg.Done()
			}
		}()
	}

	dirWg.Add(1)
	dirCh <- input.RootPath

	dirWg.Wait()
	close(dirCh)
	workerWg.Wait()

	return nil
}
