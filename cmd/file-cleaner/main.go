package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/urfave/cli/v2"

	filecleaner "github.com/ideamans/go-file-cleaner"
)

func main() {
	app := &cli.App{
		Name:      "file-cleaner",
		Usage:     "Clean up file from directory until the target size is met.",
		ArgsUsage: "<targetSize> <rootPath>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "atime",
				Aliases: []string{"a"},
				Usage:   "Use file access time instead of modified time.",
			},
			&cli.UintFlag{
				Name:    "concurrency",
				Aliases: []string{"c"},
				Value:   1,
				Usage:   "Number of concurrently. If 0, use number of CPU.",
			},
			&cli.DurationFlag{
				Name:    "time-bin",
				Aliases: []string{"b"},
				Usage:   "Time bin size for aggregation (e.g. 60m, 1h, 3600s). Default 60m",
				Value:   time.Hour,
			},
			&cli.StringFlag{
				Name:    "pattern",
				Aliases: []string{"p"},
				Usage:   "Minimatch pattern for filtering files and directories.",
			},
			&cli.Uint64Flag{
				Name:  "block-size",
				Usage: "Block size in bytes. Default 0 (auto detection in UNIX, 4096 in others).",
			},
			&cli.BoolFlag{
				Name:  "dry-run",
				Usage: "Dry run mode. Just print the file paths but do not remove.",
			},
			&cli.UintFlag{
				Name:  "examples",
				Usage: "Number of removed files to display as examples.",
			},
		},
		Action: func(c *cli.Context) error {
			var input filecleaner.CleaningInput

			if c.Args().Len() < 2 {
				cli.ShowAppHelp(c)
				return errors.New("not enough arguments: requires [targetSize] [path]")
			}

			// target size
			targetSizeStr := c.Args().Get(0)
			targetSize, err := humanize.ParseBytes(targetSizeStr)
			if err != nil {
				return fmt.Errorf("invalid target size %s: %v", targetSizeStr, err)
			}
			input.TargetSize = targetSize

			// root path
			input.RootPath = c.Args().Get(1)

			// atime
			input.FileTimeType = filecleaner.ModTime
			if c.Bool("atime") {
				input.FileTimeType = filecleaner.AccessTime
			}

			// time bin
			input.TimeBin = c.Duration("time-bin")

			// concurrency
			input.Concurrency = c.Int("concurrency")
			if input.Concurrency == 0 {
				input.Concurrency = int(runtime.NumCPU())
			}

			// pattern
			input.Pattern = c.String("pattern")

			// block size
			input.BlockSize = c.Uint64("block-size")
			if input.BlockSize == 0 {
				bs, err := filecleaner.GuessDiskBlockSize(input.RootPath)
				// ブロックサイズの推測ができなくてもログを出力して一般的な4096で続行
				if err != nil {
					log.Printf("[warning] failed to get block size: %v", err)
				}
				if bs == 0 {
					bs = 4096
				}
				input.BlockSize = bs
			}

			// dry run
			input.DryRun = c.Bool("dry-run")

			// examples
			input.Examples = c.Int("examples")

			// error handler
			input.ErrorHandler = func(rootPath, fullPath string, err error) {
				log.Printf("[error] %s: %v", fullPath, err)
			}

			// logging
			input.Logging = func(message string) {
				log.Printf("[info] %s", message)
			}

			// run cleaner
			output, err := filecleaner.Clean(&input)
			if err != nil {
				return fmt.Errorf("failed to clean %s: %v", input.RootPath, err)
			}

			// print result
			if output.RemovedFiles == 0 {
				log.Printf("[info] no files removed. disk usage is %s under %s", humanize.Bytes(output.BlockSizeAfter), humanize.Bytes(input.TargetSize))
			} else {
				log.Printf("[info] cleaned up %d files(%s). now disk usage is %s", output.RemovedFiles, humanize.Bytes(output.RemovedBlockSize), humanize.Bytes(output.BlockSizeAfter))
			}

			// examples
			for i, example := range output.Examples {
				log.Printf("[info] example[%d]: %s", i, example)
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatalf("[fatal] %s", err)
	}
}
