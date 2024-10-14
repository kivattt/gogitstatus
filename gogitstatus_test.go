package gogitstatus

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
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
		fmt.Print("Test file " + expectedPath + ": ")

		file, err := os.Open(expectedPath)
		if err != nil {
			fmt.Println(err)
			testFailed = true
			continue
		}

		var expectedChangedFiles []ChangedFile
		var expectedError error = nil

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()

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

		changedFiles, err := StatusRaw(filesPath, indexPath)
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
	printGray("TestParseGitIndex\n")

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
			fmt.Print("Test file " + expectedPath + ": ")

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
