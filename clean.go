package filecleaner

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/djherbis/atime"
	"github.com/dustin/go-humanize"
	emptydircleaner "github.com/ideamans/go-empty-dir-cleaner"
)

// ファイルクリーニング処理全体の入出力

// ファイル時刻のタイプ
type FileTimeType uint8

const (
	AccessTime FileTimeType = iota
	ModTime
)

// ファイルクリーニング処理のワークフローフェーズ
type CleaningPhase uint8

const (
	CleaningPhaseIndexing CleaningPhase = iota
	CleaningPhaseThreshold
	CleaningPhaseRemoving
	CleaningPhaseCleaningEmptyDirs
)

// ログハンドラ
type Logging func(message string)

// デフォルトログ出力
func DefaultLogger(message string) {
	log.Println(message)
}

// ファイルクリーニング処理全体の入力
type CleaningInput struct {
	RootPath     string
	BlockSize    uint64
	TargetSize   uint64
	FileTimeType FileTimeType
	TimeBin      time.Duration
	Pattern      string
	Examples     int
	Concurrency  int
	DryRun       bool
	ErrorHandler ErrorHandler
	Logging      Logging
}

// ファイルクリーニング処理全体の出力
type CleaningOutput struct {
	Phase            CleaningPhase
	BlockSizeBefore  uint64
	RemoveBefore     time.Time
	RemovedFiles     uint64
	RemovedBlockSize uint64
	BlockSizeAfter   uint64
	Examples         []string
	EmptyDirExamples []string
}

// ファイルサイズとファイル時刻の切り上げ関連

// ファイルサイズからブロックサイズへの切り上げ
func fileSizeToBlockSize(fileInfo os.FileInfo, blockSize uint64) uint64 {
	size := uint64(fileInfo.Size())
	return ((size + blockSize - 1) / blockSize) * blockSize
}

// ファイル時刻のUNIXタイムスタンプ
func fileTimeUnix(fileInfo os.FileInfo, fileTimeType FileTimeType) uint64 {
	if fileTimeType == AccessTime {
		return uint64(atime.Get(fileInfo).Unix())
	}
	return uint64(fileInfo.ModTime().Unix())
}

// ファイル時刻から集計時間単位秒への切り上げ
func fileTimeToTimeBinSec(fileInfo os.FileInfo, fileTimeType FileTimeType, timeBinSec uint64) uint64 {
	t := fileTimeUnix(fileInfo, fileTimeType)
	return ((t + timeBinSec - 1) / timeBinSec) * timeBinSec
}

// 第1フェーズ: 集計の実装

// 集計の入力
type indexingInput struct {
	rootPath     string
	concurrency  uint
	pattern      string
	fileTimeType FileTimeType
	timeBinSec   uint64
	blockSize    uint64
	errorHandler ErrorHandler
}

// 集計の出力
type indexingOutput struct {
	totalBlockSize      uint64
	blockSizesByTimeBin map[uint64]uint64
}

// 集計処理
func indexByFileTime(input *indexingInput) (*indexingOutput, error) {
	output := &indexingOutput{
		totalBlockSize:      0,
		blockSizesByTimeBin: make(map[uint64]uint64),
	}

	// ブロックサイズのインデックス集計は並列化によりMutexの要否があるためクロージャにしておく
	fileIndexing := func(totalBlockSizeDelta, fileTimeBinSec, fileBlockSizeDelta uint64) {
		if val, exist := output.blockSizesByTimeBin[fileTimeBinSec]; exist {
			output.blockSizesByTimeBin[fileTimeBinSec] = val + fileBlockSizeDelta
		} else {
			output.blockSizesByTimeBin[fileTimeBinSec] += fileBlockSizeDelta
		}
		output.totalBlockSize += totalBlockSizeDelta
	}

	dirIndexing := func(totalBlockSizeDelta uint64) {
		output.totalBlockSize += totalBlockSizeDelta
	}

	var mu sync.Mutex
	err := Crawl(&CrawlingInput{
		RootPath:    input.rootPath,
		Concurrency: input.concurrency,
		Pattern:     input.pattern,
	}, func(rootPath, fullPath string, fileInfo os.FileInfo) {
		if fileInfo.IsDir() {
			// ディレクトリの場合は1ブロックサイズ単位を合計にのみ反映
			if input.concurrency == 1 {
				dirIndexing(input.blockSize)
			} else {
				mu.Lock()
				dirIndexing(input.blockSize)
				mu.Unlock()
			}
		} else {
			// ファイルの場合、ブロックサイズと集計時間単位の倍数に切り上げ
			blockSize := fileSizeToBlockSize(fileInfo, input.blockSize)
			timeBinSec := fileTimeToTimeBinSec(fileInfo, input.fileTimeType, input.timeBinSec)

			if input.concurrency == 1 {
				fileIndexing(blockSize, timeBinSec, blockSize)
			} else {
				mu.Lock()
				fileIndexing(blockSize, timeBinSec, blockSize)
				mu.Unlock()
			}
		}
	}, func(rootPath, fullPath string, err error) {
		if input.errorHandler != nil {
			input.errorHandler(rootPath, fullPath, fmt.Errorf("error in indexing: %v", err))
		}
	})

	if err != nil {
		return nil, err
	}

	return output, nil
}

// 第2フェーズ: 閾値判定の実装

// 閾値判定の入力
type thresholdInput struct {
	targetSize          uint64
	totalBlockSize      uint64
	blockSizesByTimeBin map[uint64]uint64
}

// 閾値判定の出力
type thresholdOutput struct {
	skip            bool
	removeBeforeSec uint64
}

// 閾値判定処理
// ファイル時刻でインデックスされたブロックサイズの集計を元に、いつ以前のファイルを削除するか計算する
func threshold(input *thresholdInput) (*thresholdOutput, error) {
	output := &thresholdOutput{
		skip:            false,
		removeBeforeSec: 0,
	}

	if input.totalBlockSize <= input.targetSize {
		output.skip = true
		return output, nil
	}

	// blocSizesByTimeBinのキーを昇順ソートし、input.TargetSizeを超えるキーを発見
	var keys []uint64
	for k := range input.blockSizesByTimeBin {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	removingSize := input.totalBlockSize - input.targetSize
	var removedSize uint64
	for _, k := range keys {
		output.removeBeforeSec = k
		removedSize += input.blockSizesByTimeBin[k]
		if removedSize >= removingSize {
			break
		}
	}

	return output, nil
}

// 第3フェーズ: ファイル削除の実装

// ファイル削除の入力
type removingInput struct {
	rootPath        string
	concurrency     uint
	pattern         string
	fileTimeType    FileTimeType
	blockSize       uint64
	dryRun          bool
	timeBinSec      uint64
	examples        uint
	removeBeforeSec uint64
	errorHandler    ErrorHandler
	logging         Logging
}

// ファイル削除の出力
type removingOutput struct {
	files     uint64
	blockSize uint64
	examples  []string
}

func removeFiles(input *removingInput) (*removingOutput, error) {
	output := &removingOutput{
		files:     0,
		blockSize: 0,
		examples:  make([]string, 0, input.examples),
	}

	// 削除されたブロックサイズの集計は並列化によりMutexの要否があるためクロージャにしておく
	examples := int(input.examples)
	sumUp := func(rootPath, fullPath string, blockSizeDelta uint64) {
		output.files++
		output.blockSize += blockSizeDelta
		if len(output.examples) < examples {
			relPath, err := filepath.Rel(rootPath, fullPath)
			if err == nil {
				output.examples = append(output.examples, relPath)
			}
		}
	}

	var mu sync.Mutex
	err := Crawl(&CrawlingInput{
		RootPath:    input.rootPath,
		Concurrency: input.concurrency,
		Pattern:     input.pattern,
	}, func(rootPath, fullPath string, fileInfo os.FileInfo) {
		// ディレクトリは無視
		if fileInfo.IsDir() {
			return
		}

		fileTimeSec := fileTimeUnix(fileInfo, input.fileTimeType)
		blockSize := fileSizeToBlockSize(fileInfo, input.blockSize)
		if fileTimeSec <= input.removeBeforeSec {
			// 削除対象時刻以前のファイルを削除する
			if input.dryRun {
				// ドライランの場合はメッセージのみ
				if input.logging != nil {
					input.logging(fmt.Sprintf("dry run: remove %s (block size: %s)", fullPath, humanize.Bytes(blockSize)))
				}
			} else {
				err := os.Remove(fullPath)
				if err != nil {
					if input.errorHandler != nil {
						input.errorHandler(rootPath, fullPath, fmt.Errorf("error to remove: %v", err))
					}
					return
				}
			}

			if input.concurrency == 1 {
				sumUp(rootPath, fullPath, blockSize)
			} else {
				mu.Lock()
				sumUp(rootPath, fullPath, blockSize)
				mu.Unlock()
			}
		}
	}, func(rootPath, fullPath string, err error) {
		if input.errorHandler != nil {
			input.errorHandler(rootPath, fullPath, fmt.Errorf("error in removing: %v", err))
		}
	})

	if err != nil {
		return nil, err
	}

	return output, nil
}

func Clean(input *CleaningInput) (*CleaningOutput, error) {
	// config.Concurrencyが1未満の場合は想定しないのでassertion
	if input.Concurrency < 1 {
		return nil, fmt.Errorf("invalid concurrency: %d", input.Concurrency)
	}

	output := &CleaningOutput{}

	// ブロックサイズとファイル時刻の再利用する計算単位
	timeBinSec := uint64(input.TimeBin.Seconds())

	// 第1フェーズ: 集計
	output.Phase = CleaningPhaseIndexing
	if input.Logging != nil {
		input.Logging(fmt.Sprintf("started block size indexing by file time: %s", input.RootPath))
	}

	indexingOutput, err := indexByFileTime(&indexingInput{
		rootPath:     input.RootPath,
		concurrency:  uint(input.Concurrency),
		pattern:      input.Pattern,
		fileTimeType: input.FileTimeType,
		timeBinSec:   timeBinSec,
		blockSize:    input.BlockSize,
		errorHandler: input.ErrorHandler,
	})
	if err != nil {
		return output, err
	}

	// 事前の合計ブロックサイズを出力に反映
	output.BlockSizeBefore = indexingOutput.totalBlockSize

	// 第2フェーズ: 閾値判定
	output.Phase = CleaningPhaseThreshold
	if input.Logging != nil {
		input.Logging("started to compute file time threshold")
	}

	thresholdOutput, err := threshold(&thresholdInput{
		targetSize:          input.TargetSize,
		totalBlockSize:      indexingOutput.totalBlockSize,
		blockSizesByTimeBin: indexingOutput.blockSizesByTimeBin,
	})
	if err != nil {
		return output, err
	}

	// 閾値判定でスキップの場合は削除の必要がないのでここで終了
	if thresholdOutput.skip {
		output.BlockSizeAfter = indexingOutput.totalBlockSize
		return output, nil
	}

	// 閾値を出力に反映
	output.RemoveBefore = time.Unix(int64(thresholdOutput.removeBeforeSec), 0)

	// 第3フェーズ: ファイル削除
	output.Phase = CleaningPhaseRemoving
	if input.Logging != nil {
		input.Logging(fmt.Sprintf("started to remove files before: %s", output.RemoveBefore.Format(time.RFC3339)))
	}

	removingOutput, err := removeFiles(&removingInput{
		rootPath:        input.RootPath,
		concurrency:     uint(input.Concurrency),
		pattern:         input.Pattern,
		fileTimeType:    input.FileTimeType,
		blockSize:       input.BlockSize,
		dryRun:          input.DryRun,
		timeBinSec:      timeBinSec,
		examples:        uint(input.Examples),
		removeBeforeSec: thresholdOutput.removeBeforeSec,
		errorHandler:    input.ErrorHandler,
		logging:         input.Logging,
	})
	if err != nil {
		return output, err
	}

	// 削除ファイル数と削除ブロックサイズを出力に反映
	output.RemovedFiles = removingOutput.files
	output.RemovedBlockSize = removingOutput.blockSize
	output.Examples = removingOutput.examples
	output.BlockSizeAfter = output.BlockSizeBefore - output.RemovedBlockSize

	// dry-runの場合は空ディレクトリを削除せずここで終了
	if input.DryRun {
		return output, nil
	}

	// 第4フェーズ: 空ディレクトリの削除
	output.Phase = CleaningPhaseCleaningEmptyDirs
	if input.Logging != nil {
		input.Logging("started cleaning up empty directories")
	}

	// empty-dir-cleanerを利用
	dircleanerResult, err := emptydircleaner.Clean(&emptydircleaner.CleaningInput{
		RootPath: input.RootPath,
		Examples: uint(input.Examples),
		ErrorHandler: func(rootDir, currentDir string, err error) {
			if input.ErrorHandler != nil {
				input.ErrorHandler(rootDir, currentDir, err)
			}
		},
	})
	if err != nil {
		return output, err
	}

	// 削除されたディレクトリを加算・削除されたディレクトリ例を出力に反映
	output.RemovedBlockSize += uint64(dircleanerResult.RemovedDirs) * input.BlockSize
	output.EmptyDirExamples = dircleanerResult.Examples

	// 最終的な合計ブロックサイズを出力に反映
	output.BlockSizeAfter = output.BlockSizeBefore - output.RemovedBlockSize

	return output, nil
}
