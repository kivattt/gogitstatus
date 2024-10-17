//go:build !windows
// +build !windows

package gogitstatus

import (
	"hash"
	"io/fs"
	"os"
	"syscall"
)

// Writes the data stored in file into hash
func sha1HashSum(hash hash.Hash, file *os.File, stat fs.FileInfo) error {
	if stat.Size() > 0 {
		data, err := syscall.Mmap(int(file.Fd()), 0, int(stat.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
		if err != nil {
			return err
		}

		hash.Write(data)

		err = syscall.Munmap(data)
		if err != nil {
			return err
		}
	}

	return nil
}
