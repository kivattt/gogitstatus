package gogitstatus

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Operation int
const (
	NewFile Operation = 0
	Added = 1
	Removed = 2
	Renamed = 3
)

// Returns "unknown" if given an invalid operation value
func OperationToString(operation Operation) string {
	switch operation {
	case NewFile:
		return "new file"
	case Added:
		return "added"
	case Removed:
		return "removed"
	case Renamed:
		return "renamed"
	}

	return "unknown"
}

type FileStatus struct {
	entry fs.DirEntry
	operation Operation
	oldNameIfRenamed string
}

// Simplified, we only care about the relative path and hash
type gitIndexEntry struct {
	path string
	hash []byte // 20 bytes for the standard SHA-1
}

func readIndexEntryPathName(file *os.File) (string, error) {
	var ret strings.Builder

	singleByteSlice := make([]byte, 1)
	for {
		_, err := io.ReadFull(file, singleByteSlice)
		if err != nil {
			return "", errors.New("Invalid size, readIndexEntryPathName failed: " + err.Error())
		}

		b := singleByteSlice[0]

		if b == 0 {
			for i := 0; i < 7; i++ {
				_, err := io.ReadFull(file, singleByteSlice)
				if err != nil {
					return "", errors.New("Invalid size, readIndexEntryPathName failed while iterating over null bytes: " + err.Error())
				}

				if singleByteSlice[0] != 0 {
					file.Seek(-1, 1)
					break
				}
			}
			break
		} else {
			ret.WriteByte(b)
		}
	}

	return ret.String(), nil
}

// Git Index file format version 2
// https://git-scm.com/docs/index-format
func parseGitIndex(path string) ([]gitIndexEntry, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !stat.Mode().IsRegular() {
		return nil, errors.New("Not a regular file")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	headerBytes := make([]byte, 12)
	_, err = io.ReadFull(file, headerBytes)
	if err != nil {
		return nil, err
	}

	if !bytes.HasPrefix(headerBytes, []byte{'D', 'I', 'R', 'C'}) {
		return nil, errors.New("Invalid header, missing \"DIRC\"")
	}

	version := binary.BigEndian.Uint32(headerBytes[4:8])
	if version != 2 {
		return nil, errors.New("Unsupported version: " + strconv.FormatInt(int64(version), 10))
	}

	numEntries := binary.BigEndian.Uint32(headerBytes[8:12])
	fmt.Println("num entries:", numEntries)

	entries := make([]gitIndexEntry, numEntries)

	var entryIndex uint32
	for entryIndex = 0; entryIndex < numEntries; entryIndex++ {
		// Seek to "object name" (hash data)
		_, err := file.Seek(42, 1) // 336 bits
		if err != nil {
			return nil, errors.New("Invalid size, unable to seek within entry at index " + strconv.FormatInt(int64(entryIndex), 10))
		}

		// Read hash data
		entries[entryIndex].hash = make([]byte, 20)
		_, err = io.ReadFull(file, entries[entryIndex].hash)
		if err != nil {
			return nil, errors.New("Invalid size, unable to read 20-byte SHA-1 hash at index " + strconv.FormatUint(uint64(entryIndex), 10))
		}

		// Seek to entry path name
		/*_, err = file.Seek(6, 1) // 48 bits
		if err != nil {
			return nil, errors.New("Invalid size, unable to seek within entry at index " + strconv.FormatInt(int64(entryIndex), 10))
		}*/

		// Read variable-length path name
		pathName, err := readIndexEntryPathName(file)
		if err != nil {
			return nil, err
		}

		entries[entryIndex].path = pathName
	}

	return entries, nil
}

func ignoreEntry(entry fs.DirEntry) bool {
	if entry.Name() == ".git" {
		return true
	}

	return false
}

// Takes in the path of a local git repository and returns the list of changed (uncommited) files, or an error
func Status(path string) ([]FileStatus, error) {
	gitPath := filepath.Join(path, ".git")
	indexPath := filepath.Join(path, ".git", "index")

	stat, err := os.Stat(gitPath)
	if err != nil || !stat.IsDir() {
		return nil, errors.New("Not a Git repository")
	}

	_, err = os.Stat(indexPath)
	// If .git/index file is missing, all files are "new file"
	if err != nil {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}

		var ret []FileStatus
		for _, e := range entries {
			if !ignoreEntry(e) {
				ret = append(ret, FileStatus{entry: e, operation: NewFile})
			}
		}
		return ret, nil
	}

	indexEntries, err := parseGitIndex(filepath.Join(path, ".git", "index"))
	if err != nil {
		return nil, errors.New("Unable to read " + indexPath + ": " + err.Error())
	}

	fmt.Println("INDEX ENTRIES:")
	for _, e := range indexEntries {
		fmt.Println("path: " + string(e.path) + ", hash: " + hex.EncodeToString(e.hash))
	}

	return nil, nil
}
