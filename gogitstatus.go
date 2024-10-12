package gogitstatus

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// A small subset of a Git index entry, only the relative path and 20-byte SHA-1 hash data
type GitIndexEntry struct {
	Path string
	Hash []byte // 20 bytes for the standard SHA-1
}

func readIndexEntryPathName(file *os.File) (string, error) {
	var ret strings.Builder

	// Entry length so far
	entryLength := 40 + 20 + 2

	singleByteSlice := make([]byte, 1)
	for {
		_, err := io.ReadFull(file, singleByteSlice)
		if err != nil {
			return "", errors.New("Invalid size, readIndexEntryPathName failed: " + err.Error())
		}

		b := singleByteSlice[0]

		if b == 0 {
			break
		} else {
			ret.WriteByte(b)
			entryLength++
		}
	}

	// Read up to 7 extra null padding bytes
	n := 8 - (entryLength % 8)
	if n == 0 {
		n = 8
	}
	n-- // We already read 1 null byte

	b := make([]byte, n)
	_, err := io.ReadFull(file, b)
	if err != nil {
		return "", errors.New("Invalid size, readIndexEntryPathName failed while seeking over null bytes: " + err.Error())
	}

	for _, e := range b {
		if e != 0 {
			return "", errors.New("Non-null byte found in null padding of length " + strconv.Itoa(n))
		}
	}

	return ret.String(), nil
}

// Parses a Git Index file (version 2)
// See file format: https://git-scm.com/docs/index-format
func ParseGitIndex(path string) ([]GitIndexEntry, error) {
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
	entries := make([]GitIndexEntry, numEntries)

	var entryIndex uint32
	for entryIndex = 0; entryIndex < numEntries; entryIndex++ {
		// Seek to "object name" (hash data)
		_, err := file.Seek(40, 1)
		if err != nil {
			return nil, errors.New("Invalid size, unable to seek to object name within entry at index " + strconv.FormatInt(int64(entryIndex), 10))
		}

		// Read hash data
		entries[entryIndex].Hash = make([]byte, 20)
		_, err = io.ReadFull(file, entries[entryIndex].Hash)
		if err != nil {
			return nil, errors.New("Invalid size, unable to read 20-byte SHA-1 hash at index " + strconv.FormatUint(uint64(entryIndex), 10))
		}

		// Seek to entry path name
		_, err = file.Seek(2, 1)
		if err != nil {
			return nil, errors.New("Invalid size, unable to seek to path name within entry at index " + strconv.FormatInt(int64(entryIndex), 10))
		}

		// Read variable-length path name
		pathName, err := readIndexEntryPathName(file)
		if err != nil {
			return nil, err
		}

		entries[entryIndex].Path = pathName
	}

	return entries, nil
}

func ignoreEntry(entry fs.DirEntry) bool {
	if entry.Name() == ".git" {
		return true
	}

	return false
}

func hashMatches(path string, hash []byte) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	stat, err := os.Stat(path)
	if err != nil {
		return false
	}

	newHash := sha1.New()
	_, err = newHash.Write(append([]byte("blob "+strconv.FormatInt(stat.Size(), 10)), 0))
	if err != nil {
		return false
	}

	_, err = io.Copy(newHash, file) // TODO: Check if written size is same as stat.Size() ?
	if err != nil {
		return false
	}

	// Debugging
	/*bool2Str := func(b bool) string {
		if b {
			return "\x1b[32m true\x1b[0m"
		}
		return "\x1b[31mfalse\x1b[0m"
	}

	b := reflect.DeepEqual(hash, newHash.Sum(nil))
	fmt.Println("hash: " + hex.EncodeToString(hash) + ", newHash: " + hex.EncodeToString(newHash.Sum(nil)) + ", matches? " + bool2Str(b) + ", " + path)*/

	return reflect.DeepEqual(hash, newHash.Sum(nil))
}

// Takes in the path of a local git repository and returns the list of changed (unstaged/untracked) files in filepaths relative to path, or an error.
func Status(path string) ([]string, error) {
	dotGitPath := filepath.Join(path, ".git")
	stat, err := os.Stat(dotGitPath)
	if err != nil || !stat.IsDir() {
		return nil, errors.New("Not a Git repository")
	}

	return StatusRaw(path, filepath.Join(dotGitPath, "index"))
}

// Does not check if path is a valid git repository
func StatusRaw(path string, gitIndexPath string) ([]string, error) {
	_, err := os.Stat(gitIndexPath)
	// If git index file is missing, all files are unstaged/untracked
	if err != nil {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}

		var paths []string
		for _, e := range entries {
			if !ignoreEntry(e) {
				paths = append(paths, filepath.Join(path, e.Name()))
			}
		}

		return paths, nil
	}

	indexEntries, err := ParseGitIndex(gitIndexPath)
	if err != nil {
		return nil, errors.New("Unable to read " + gitIndexPath + ": " + err.Error())
	}

	stat, err := os.Stat(path)
	if err != nil || !stat.IsDir() {
		return nil, errors.New("Path does not exist: " + path)
	}

	var paths []string
	// Accumulate all not-ignored paths
	err = filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
		if ignoreEntry(d) {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		paths = append(paths, filePath)
		return nil
	})

	for _, entry := range indexEntries {
		if hashMatches(filepath.Join(path, entry.Path), entry.Hash) {
			pathFound := slices.Index(paths, filepath.Join(path, entry.Path))
			if pathFound == -1 {
				continue
			}

			paths = slices.Delete(paths, pathFound, pathFound+1)
		}
	}

	return paths, nil
}
