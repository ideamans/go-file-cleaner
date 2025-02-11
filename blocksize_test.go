package filecleaner

import (
	"os"
	"runtime"
	"testing"
)

// ブロックサイズのテスト
func TestBlockSize(t *testing.T) {
	temp, err := os.MkdirTemp("", "blocksize_test")
	if err != nil {
		t.Fatal(err)
	}

	bs, err := GuessDiskBlockSize(temp)

	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly", "darwin":
		if err != nil {
			t.Errorf("failed to get block size: %v", err)
		}
		if bs == 0 {
			t.Errorf("unexpected block size: %v", bs)
		}
	default:
		if err == nil {
			t.Errorf("unexpected success: %v", bs)
		}
		if bs != 4096 {
			t.Errorf("unexpected block size: %v", bs)
		}
	}
}
