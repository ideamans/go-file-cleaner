//go:build !js && !(linux || freebsd || openbsd || netbsd || dragonfly || darwin)

package filecleaner

import (
	"fmt"
	"runtime"
)

var (
	defaultBlockSize = uint64(4096)
)

// UNIX以外のプラットフォームではデフォルトブロックサイズを返す
func GuessDiskBlockSize(path string) (uint64, error) {
	return defaultBlockSize, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
}
