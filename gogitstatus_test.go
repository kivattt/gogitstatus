package gogitstatus

import (
	"archive/zip"
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func printRed(text string) {
	if runtime.GOOS == "windows" {
		fmt.Print(text)
	} else {
		fmt.Print("\x1b[31m" + text + "\x1b[0m")
	}
}

func printGreen(text string) {
	if runtime.GOOS == "windows" {
		fmt.Print(text)
	} else {
		fmt.Print("\x1b[32m" + text + "\x1b[0m")
	}
}

func printGray(text string) {
	if runtime.GOOS == "windows" {
		fmt.Print(text)
	} else {
		fmt.Print("\x1b[1;30m" + text + "\x1b[0m")
	}
}

// Copied from: https://stackoverflow.com/a/24792688
func extractZipArchive(zipFilePath, destination string) error {
	r, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return err
	}

	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	os.MkdirAll(destination, 0755)
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}

		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(destination, f.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(destination)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}

			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func TestStatusRaw(t *testing.T) {
	testsPath := "./tests-statusraw"
	tests, err := os.ReadDir(testsPath)
	if err != nil {
		t.Fatal(err)
	}

	printGray("TestStatusRaw:\n")

	printChangedFiles := func(entries []ChangedFile) {
		for _, e := range entries {
			fmt.Println("    " + WhatChangedToString(e.WhatChanged) + " " + e.Path)
		}
	}

	testFailed := false

	for _, test := range tests {
		filesPath := filepath.Join(testsPath, test.Name(), "files")
		indexPath := filepath.Join(testsPath, test.Name(), "index")
		expectedPath := filepath.Join(testsPath, test.Name(), "expected.txt")
		if runtime.GOOS == "windows" {
			expectedWindowsPath := filepath.Join(testsPath, test.Name(), "expected_windows.txt")
			_, err := os.Stat(expectedWindowsPath)
			if err == nil {
				expectedPath = expectedWindowsPath
			}
		}
		fmt.Print("Test " + expectedPath + ": ")

		file, err := os.Open(expectedPath)
		if err != nil {
			fmt.Println(err)
			testFailed = true
			continue
		}

		var expectedChangedFiles []ChangedFile
		var expectedError error = nil
		var expectedAnyError bool

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "Any error") {
				expectedAnyError = true
				break
			}

			if strings.HasPrefix(line, "Error text:") {
				expectedError = errors.New(line[len("Error text:"):])
				break
			}

			split := strings.Split(line, " ")
			var whatChangedText string
			var pathText string
			if len(split) < 2 {
				whatChangedText = ""
				pathText = split[0]
			} else {
				whatChangedText = split[0]
				pathText = split[1]
			}

			expectedChangedFiles = append(expectedChangedFiles, ChangedFile{Path: filepath.Join(filesPath, pathText), WhatChanged: StringToWhatChanged(whatChangedText)})
		}

		_, err = os.Stat(filesPath)
		if err != nil {
			zipFilePath := filepath.Join(testsPath, test.Name(), "files.zip")
			err := extractZipArchive(zipFilePath, filesPath)
			if err == nil {
				defer func() {
					os.RemoveAll(filesPath)
				}()
			}
		}
		changedFiles, err := StatusRaw(filesPath, indexPath)

		if expectedAnyError && err == nil {
			fmt.Println("expected any error, but got nil")
			testFailed = true
			continue
		}

		if !expectedAnyError && expectedError == nil && err != nil {
			printRed("Failed\n")
			fmt.Println("expected no error, but got: " + err.Error())
			testFailed = true
			continue
		}

		if !expectedAnyError && err != nil && expectedError != nil {
			if err.Error() != expectedError.Error() {
				printRed("Failed\n")
				fmt.Println("expected error text \"" + expectedError.Error() + "\", but got: \"" + err.Error() + "\"")
				testFailed = true
				continue
			}
		}

		if !(len(changedFiles) == 0 && len(expectedChangedFiles) == 0) && !reflect.DeepEqual(changedFiles, expectedChangedFiles) {
			printRed("Failed\n")

			fmt.Println("Expected entries:")
			printChangedFiles(expectedChangedFiles)
			fmt.Println("But got:")
			printChangedFiles(changedFiles)
			testFailed = true
			continue
		}

		printGreen("Success\n")
	}

	if testFailed {
		t.Fatal("See above ^")
	}
}

func TestParseGitIndex(t *testing.T) {
	testsPath := "./tests-index-parser"
	tests, err := os.ReadDir(testsPath)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println()
	printGray("TestParseGitIndex:\n")

	printEntries := func(entries []GitIndexEntry) {
		for _, e := range entries {
			fmt.Println("    "+hex.EncodeToString(e.Hash), e.Path)
		}
	}

	gitIndexEntriesMatch := func(a, b []GitIndexEntry) bool {
		if len(a) != len(b) {
			return false
		}

		for i := 0; i < len(a); i++ {
			left, right := a[i], b[i]
			if !reflect.DeepEqual(left.Hash, right.Hash) {
				return false
			}

			if left.Path != right.Path {
				return false
			}
		}
		return true
	}

	testFailed := false

	for _, version := range tests {
		versionTests, err := os.ReadDir(filepath.Join(testsPath, version.Name()))
		if err != nil {
			fmt.Println(err)
			testFailed = true
			continue
		}

		for _, versionTest := range versionTests {
			indexPath := filepath.Join(testsPath, version.Name(), versionTest.Name(), "index")
			expectedPath := filepath.Join(testsPath, version.Name(), versionTest.Name(), "expected.txt")
			fmt.Print("Test " + expectedPath + ": ")

			file, err := os.Open(expectedPath)
			if err != nil {
				printRed("Failed\n")
				fmt.Println(err)
				testFailed = true
				continue
			}

			expectedEntries := []GitIndexEntry{}
			var expectedError error = nil

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "Error text:") {
					expectedEntries = []GitIndexEntry{}
					expectedError = errors.New(line[len("Error text:"):])
					break
				}

				sha1HashHex := line[:40]
				pathName := line[41:]

				sha1HashBytes, err := hex.DecodeString(sha1HashHex)
				if err != nil {
					printRed("Failed\n")
					fmt.Println(err)
					testFailed = true
					continue
				}

				expectedEntries = append(expectedEntries, GitIndexEntry{Path: pathName, Hash: sha1HashBytes})
			}
			file.Close()

			entries, err := ParseGitIndex(indexPath)
			if expectedError == nil && err != nil {
				printRed("Failed\n")
				fmt.Println("expected no error, but got: " + err.Error())
				testFailed = true
				continue
			}

			if err != nil && expectedError != nil {
				if err.Error() != expectedError.Error() {
					printRed("Failed\n")
					fmt.Println("expected error text \"" + expectedError.Error() + "\", but got: \"" + err.Error() + "\"")
					testFailed = true
					continue
				}
			}

			if !gitIndexEntriesMatch(entries, expectedEntries) {
				printRed("Failed\n")

				fmt.Println("Expected entries:")
				printEntries(expectedEntries)
				fmt.Println("But got:")
				printEntries(entries)
				testFailed = true
				continue
			}

			printGreen("Success\n")
		}
	}

	if testFailed {
		t.Fatal("See above ^")
	}
}
