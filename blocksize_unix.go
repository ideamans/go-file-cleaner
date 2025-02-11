//go:build !js && (linux || freebsd || openbsd || netbsd || dragonfly || darwin)

package filecleaner

import (
	"golang.org/x/sys/unix"
)

var (
	defaultBlockSize = uint64(4096)
)

// UNIX系プラットフォームではunixモジュールを用いてディスクのブロックサイズを推測する
func GuessDiskBlockSize(path string) (uint64, error) {
	// Linux や BSD 系, macOS など UNIX ベースであれば x/sys/unix の Statfs を利用
	var statfs unix.Statfs_t
	if err := unix.Statfs(path, &statfs); err != nil {
		return defaultBlockSize, err
	}
	return uint64(statfs.Bsize), nil
}
