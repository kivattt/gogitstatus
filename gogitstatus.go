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
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/sabhiram/go-gitignore"
)

// A small subset of a Git index entry, only 32-bit mode and 20-byte SHA-1 hash data
type GitIndexEntry struct {
	Mode uint32 // Contains the file type and unix permission bits
	Hash []byte // 20 bytes for the standard SHA-1
}

// TODO: Can speed this up by first reading 0xfff bytes, and then 8 bytes at a time until the last byte of the 8-byte section is a null byte
func readIndexEntryPathName(file *os.File) (strings.Builder, error) {
	var ret strings.Builder

	// Entry length so far
	entryLength := 40 + 20 + 2

	singleByteSlice := make([]byte, 1)
	for {
		_, err := io.ReadFull(file, singleByteSlice)
		if err != nil {
			return ret, errors.New("Invalid size, readIndexEntryPathName failed: " + err.Error())
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
		return ret, errors.New("Invalid size, readIndexEntryPathName failed while seeking over null bytes: " + err.Error())
	}

	for _, e := range b {
		if e != 0 {
			return ret, errors.New("Non-null byte found in null padding of length " + strconv.Itoa(n))
		}
	}

	return ret, nil
}

// Returns the relative paths mapping to the GitIndexEntry
// Parses a Git Index file (version 2)
func ParseGitIndex(path string) (map[string]GitIndexEntry, error) {
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
	entries := make(map[string]GitIndexEntry)

	var entryIndex uint32
	for entryIndex = 0; entryIndex < numEntries; entryIndex++ {
		// Seek to 32-bit mode
		_, err := file.Seek(24, 1) // 192 bits
		if err != nil {
			return nil, errors.New("Invalid size, unable to seek to 32-bit mode within entry at index " + strconv.FormatInt(int64(entryIndex), 10))
		}

		// Read 32-bit mode
		bytes := make([]byte, 4) // 32 bits
		_, err = io.ReadFull(file, bytes)
		if err != nil {
			return nil, errors.New("Invalid size, unable to read 32-bit mode within entry at index " + strconv.FormatInt(int64(entryIndex), 10))
		}

		mode := binary.BigEndian.Uint32(bytes)

		// Seek to "object name" (hash data)
		_, err = file.Seek(12, 1) // 96 bits
		if err != nil {
			return nil, errors.New("Invalid size, unable to seek to object name within entry at index " + strconv.FormatInt(int64(entryIndex), 10))
		}

		// Read hash data
		hash := make([]byte, 20) // 160 bits
		_, err = io.ReadFull(file, hash)
		if err != nil {
			return nil, errors.New("Invalid size, unable to read 20-byte SHA-1 hash at index " + strconv.FormatUint(uint64(entryIndex), 10))
		}

		flagsBytes := make([]byte, 2) // 16 bits 'flags' field
		_, err = io.ReadFull(file, flagsBytes)
		if err != nil {
			return nil, errors.New("Invalid size, unable to read 2-byte flags field at index " + strconv.FormatUint(uint64(entryIndex), 10))
		}

		flags := binary.BigEndian.Uint16(flagsBytes)
		nameLength := flags & 0xfff

		var pathName strings.Builder
		if nameLength == 0xfff { // Path name length >= 0xfff, need to manually find null bytes
			// Read variable-length path name
			pathName, err = readIndexEntryPathName(file)
			if err != nil {
				return nil, err
			}
		} else {
			bytes := make([]byte, nameLength)
			_, err := io.ReadFull(file, bytes)
			if err != nil {
				return nil, errors.New("Invalid size, unable to read path name of size " + strconv.FormatUint(uint64(nameLength), 10) + " at index " + strconv.FormatUint(uint64(entryIndex), 10))
			}

			pathName.Write(bytes)
			entryLength := 40 + 20 + 2 // Entry length so far
			// Read up to 8 null padding bytes
			n := 8 - ((int(nameLength) + entryLength) % 8)
			if n == 0 {
				n = 8
			}

			b := make([]byte, n)
			_, err = io.ReadFull(file, b)
			if err != nil {
				return nil, errors.New("Invalid size, unable to read path name null bytes of size " + strconv.FormatUint(uint64(n), 10) + " at index " + strconv.FormatUint(uint64(entryIndex), 10))
			}

			for _, e := range b {
				if e != 0 {
					return nil, errors.New("Non-null byte found in null padding of length " + strconv.FormatUint(uint64(n), 10))
				}
			}
		}

		entries[pathName.String()] = GitIndexEntry{Mode: mode, Hash: hash}
	}

	return entries, nil
}

func hashMatches(path string, hash []byte) bool {
	stat, err := os.Lstat(path)
	if err != nil {
		return false
	}

	// Symlinks are hashed with the target path, not the data of the target file
	// On Windows, symlinks are stored as regular files (with target path as the file data), so we handle them as such later
	if runtime.GOOS != "windows" && stat.Mode()&os.ModeSymlink != 0 /*|| !stat.Mode().IsRegular()*/ {
		newHash := sha1.New()
		targetPath, err := os.Readlink(path)
		if err != nil {
			return false
		}

		newHash.Write(append([]byte("blob "+strconv.Itoa(len(targetPath))), 0))
		newHash.Write([]byte(targetPath))
		return reflect.DeepEqual(hash, newHash.Sum(nil))
	}

	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	stat, err = os.Stat(path)
	if err != nil {
		return false
	}

	newHash := sha1.New()
	_, err = newHash.Write(append([]byte("blob "+strconv.FormatInt(stat.Size(), 10)), 0))
	if err != nil {
		return false
	}

	err = writeFileToHash(newHash, file, stat)
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

type WhatChanged int

const (
	// https://github.com/git/git/blob/ef8ce8f3d4344fd3af049c17eeba5cd20d98b69f/statinfo.h#L35
	MTIME_CHANGED WhatChanged = 0x0001
	CTIME_CHANGED             = 0x0002
	OWNER_CHANGED             = 0x0004
	MODE_CHANGED              = 0x0008
	INODE_CHANGED             = 0x0010 // Use or not?
	DATA_CHANGED              = 0x0020
	TYPE_CHANGED              = 0x0040
)

var whatChangedToStringMap = map[WhatChanged]string{
	MTIME_CHANGED: "MTIME_CHANGED",
	CTIME_CHANGED: "CTIME_CHANGED",
	OWNER_CHANGED: "OWNER_CHANGED",
	MODE_CHANGED:  "MODE_CHANGED",
	INODE_CHANGED: "INODE_CHANGED",
	DATA_CHANGED:  "DATA_CHANGED",
	TYPE_CHANGED:  "TYPE_CHANGED",
}

var stringToWhatChangedMap = map[string]WhatChanged{
	"MTIME_CHANGED": MTIME_CHANGED,
	"CTIME_CHANGED": CTIME_CHANGED,
	"OWNER_CHANGED": OWNER_CHANGED,
	"MODE_CHANGED":  MODE_CHANGED,
	"INODE_CHANGED": INODE_CHANGED,
	"DATA_CHANGED":  DATA_CHANGED,
	"TYPE_CHANGED":  TYPE_CHANGED,
}

func WhatChangedToString(whatChanged WhatChanged) string {
	var masksMatched []string

	for k, v := range whatChangedToStringMap {
		if whatChanged&k != 0 {
			masksMatched = append(masksMatched, v)
		}
	}

	return strings.Join(masksMatched, ",")
}

func StringToWhatChanged(text string) WhatChanged {
	split := strings.Split(text, ",")
	var ret WhatChanged
	for _, e := range split {
		ret |= stringToWhatChangedMap[e]
	}
	return ret
}

const OBJECT_TYPE_MASK = 0b1111 << 12

const REGULAR_FILE = 0b1000 << 12
const SYMBOLIC_LINK = 0b1010 << 12
const GITLINK = 0b1110 << 12

// If you pass this a nil value for stat, it will return 0
// https://github.com/git/git/blob/ef8ce8f3d4344fd3af049c17eeba5cd20d98b69f/read-cache.c#L307
func fileChanged(entry GitIndexEntry, entryFullPath string, stat os.FileInfo) WhatChanged {
	if stat == nil {
		return 0 // Deleted file
	}

	var whatChanged WhatChanged

	switch entry.Mode & OBJECT_TYPE_MASK {
	case REGULAR_FILE:
		if !stat.Mode().IsRegular() {
			whatChanged |= TYPE_CHANGED
		}

		// https://github.com/git/git/blob/ef8ce8f3d4344fd3af049c17eeba5cd20d98b69f/read-cache.c#L317
		// Windows only stores the mode permission bits in .git/index, not on disk
		if runtime.GOOS != "windows" && fs.FileMode(entry.Mode)&fs.ModePerm&0100 != stat.Mode()&fs.ModePerm&0100 {
			whatChanged |= MODE_CHANGED
		}
	case SYMBOLIC_LINK:
		// Symbolic links are stored as regular files on Windows
		if runtime.GOOS != "windows" && stat.Mode()&os.ModeSymlink == 0 /*&& !stat.Mode().IsRegular()*/ {
			whatChanged |= TYPE_CHANGED
		}
	case GITLINK:
		if !stat.IsDir() {
			whatChanged |= TYPE_CHANGED
		}
		return whatChanged
	default:
		panic("Unknown git index entry mode:" + strconv.FormatInt(int64(entry.Mode), 10))
	}

	// TODO: Store mtime and ctime to check for change here, as is done in the match_stat_data() function in Git

	if !hashMatches(entryFullPath, entry.Hash) {
		whatChanged |= DATA_CHANGED
	}

	return whatChanged
}

type ChangedFile struct {
	Path        string
	WhatChanged WhatChanged
	Untracked   bool // true = Untracked, false = Unstaged
}

// Recursively iterates through the directory path, returning a list of all the filepaths found, ignoring files/directories named ".git" and untracked files ignored by .gitignore
func AccumulatePathsNotIgnored(path string, indexEntries map[string]GitIndexEntry, respectGitIgnore bool) ([]ChangedFile, error) {
	var ignores *ignore.GitIgnore
	if respectGitIgnore {
		var err error
		// FIXME: Use exclude files priority https://git-scm.com/docs/gitignore
		ignores, err = ignore.CompileIgnoreFile(filepath.Join(path, ".gitignore"))
		if err != nil {
			ignores = nil
		}
	}

	var paths []ChangedFile
	err := filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// If it's in the .git/index, it's tracked
		_, tracked := indexEntries[filePath]

		// Don't add untracked ignored files
		if ignores != nil && !tracked {
			// We need to ignore based on the path relative to path
			rel, err := filepath.Rel(path, filePath)
			if err != nil {
				return nil
			}

			if rel == "." {
				return nil
			}

			if ignores.MatchesPath(rel) {
				if d.IsDir() {
					return filepath.SkipDir
				} else {
					return nil
				}
			}
		}

		// git status seems to ignore any file/directory named ".git", regardless of its parent directory
		if d.Name() == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			} else {
				return nil
			}
		}

		if d.IsDir() {
			return nil
		}

		paths = append(paths, ChangedFile{Path: filePath, Untracked: !tracked})
		return nil
	})

	if err != nil {
		return nil, err
	}

	return paths, nil
}

// Takes in the path of a local git repository and returns the list of changed (unstaged/untracked) files in filepaths relative to path, or an error.
func Status(path string) ([]ChangedFile, error) {
	dotGitPath := filepath.Join(path, ".git")
	stat, err := os.Stat(dotGitPath)
	if err != nil || !stat.IsDir() {
		return nil, errors.New("Not a Git repository")
	}

	return StatusRaw(path, filepath.Join(dotGitPath, "index"), true)
}

// Does not check if path is a valid git repository
func StatusRaw(path string, gitIndexPath string, respectGitIgnore bool) ([]ChangedFile, error) {
	stat, err := os.Stat(path)
	if err != nil || !stat.IsDir() {
		return nil, errors.New("Path does not exist: " + path)
	}

	// If .git/index file is missing, all files are unstaged/untracked
	_, err = os.Stat(gitIndexPath)
	if err != nil {
		return AccumulatePathsNotIgnored(path, make(map[string]GitIndexEntry), respectGitIgnore)
	}

	indexEntries, err := ParseGitIndex(gitIndexPath)
	if err != nil {
		return nil, errors.New("Unable to read " + gitIndexPath + ": " + err.Error())
	}

	paths, err := AccumulatePathsNotIgnored(path, indexEntries, respectGitIgnore)
	if err != nil {
		return nil, err
	}

	// Filter unchanged files
	for p, entry := range indexEntries {
		thePath := filepath.Join(path, p)

		pathFound := slices.IndexFunc(paths, func(e ChangedFile) bool {
			return e.Path == thePath
		})

		stat, statErr := os.Lstat(thePath)
		if statErr != nil {
			stat = nil // Just to be sure
		}

		whatChanged := fileChanged(entry, thePath, stat)

		if pathFound != -1 {
			if statErr != nil || whatChanged == 0 {
				paths = slices.Delete(paths, pathFound, pathFound+1)
			} else {
				paths[pathFound].WhatChanged = whatChanged
				paths[pathFound].Untracked = false
			}
		} else {
			// File is tracked, but ignored so we didn't add it previously. This might cause bugs?

			// Deleted files need to be added to the list since we previously only added files that already exist on the filesystem
			if statErr != nil {
				paths = append(paths, ChangedFile{Path: thePath, Untracked: false})
			}
		}
	}

	return paths, nil
}
