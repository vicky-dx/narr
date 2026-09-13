package storage

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Exists checks whether a file exists and is not empty.
func Exists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > 0
}

// RenameWithRetry renames oldPath to newPath, retrying on Windows sharing violations / access errors
// which frequently happen right after closing a large file when antivirus (Windows Defender)
// or search indexers hold a temporary lock on the file.
func RenameWithRetry(oldPath, newPath string) error {
	var err error
	for i := 0; i < 15; i++ {
		err = os.Rename(oldPath, newPath)
		if err == nil {
			return nil
		}
		time.Sleep(time.Duration(25*(i+1)) * time.Millisecond)
	}
	return err
}

// WriteAtomic safely writes data to a temporary file and renames it to the target path.
func WriteAtomic(targetPath string, header []byte, body io.Reader) (int64, error) {
	tempPath := fmt.Sprintf("%s.%d.downloading", targetPath, time.Now().UnixNano())
	out, err := os.Create(tempPath)
	if err != nil {
		return 0, err
	}

	if len(header) > 0 {
		if _, err := out.Write(header); err != nil {
			out.Close()
			_ = os.Remove(tempPath)
			return 0, err
		}
	}

	n, err := io.Copy(out, body)
	out.Close()
	if err != nil {
		_ = os.Remove(tempPath)
		return 0, err
	}

	if err := RenameWithRetry(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return 0, err
	}

	return n + int64(len(header)), nil
}

