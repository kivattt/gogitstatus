//go:build windows
// +build windows

package gogitstatus

import (
	"hash"
	"io"
	"io/fs"
	"os"
)

// Writes the data stored in file into hash
func writeFileToHash(hash hash.Hash, file *os.File, stat fs.FileInfo) error {
	_, err := io.Copy(hash, file)
	return err
}
