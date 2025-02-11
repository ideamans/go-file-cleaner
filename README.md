# File Cleaning Based on Target Disk Usage

A file cleaning tool that considers the occupied block size on disk.

Cache files and backups increase with system operation. Use this tool to periodically delete these files to prevent disk space from running out.

## Usage

Run the following command to delete files in the `/var/cache` directory in order of oldest modification time until the disk usage is below 80GB.

```bash
file-cleaner 80gb /var/cache
```

## Arguments and Options

```bash
file-cleaner [options] <targetSize> <rootPath>
```

- `<targetSize>` Specify the target disk usage (e.g., `80gb`, `100mb`). This argument is required.
- `<rootPath>` Specify the directory to clean up. This argument is required.

The following options can be specified in `[options]`:

- `-a`, `--atime` Delete old files based on the last access time instead of the last modification time.
- `-c`, `--concurrency` Execute directory traversal and file deletion in parallel. If 0 is specified, the number of CPU cores will be used.
- `-b`, `--time-bin` Specify the unit for rounding up the last time of files to save memory during aggregation and sorting (e.g., `1h`, `60m`). The default is 1 hour.
- `-p`, `--pattern` Specify the pattern of files to be deleted in the target directory (e.g., `*/backup/**`).
- `--block-size` Specify the block size on disk in bytes. The default is `4096`, and auto-detection is attempted on UNIX.
- `--dry-run` Display the files to be deleted without actually deleting them.
- `--examples` Display the specified number of examples of deleted files.

The pattern evaluation by `-p` uses [doublestar](https://github.com/bmatcuk/doublestar).

## Using as a Library

`file-cleaner` can also be used as a library. You can import and use it as follows:

```go
package main

import (
  filecleaner "github.com/ideamans/go-file-cleaner"
)

func main() {
  output, err := filecleaner.Clean(filecleaner.CleaningInput{
    TargetSize: 1024 * 1024 * 1024 * 80,
    RootPath:   "/var/cache",
  })

  fmt.Println(output)

  if err != nil {
    panic(err)
  }
}
```
