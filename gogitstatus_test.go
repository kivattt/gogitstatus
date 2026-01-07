package gogitstatus

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
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
// Modified to support symlinks (except on Windows)
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
		} else if f.Mode()&os.ModeSymlink != 0 && runtime.GOOS != "windows" { // Don't do on Windows?
			rc, err := f.Open()
			if err != nil {
				return err
			}

			targetPath, err := io.ReadAll(rc)
			if err != nil {
				return err
			}

			if err := rc.Close(); err != nil {
				return err
			}

			err = os.Symlink(string(targetPath), path)
			if err != nil {
				return err
			}
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

func getNumberFromFolderName(folderName string) (int, error) {
	numberString := ""
	for _, c := range folderName {
		if c == '_' {
			break
		}
		numberString += string(c)
	}
	return strconv.Atoi(numberString)
}

func TestStatus(t *testing.T) {
	testsPath := "./tests-status"
	tests, err := os.ReadDir(testsPath)
	if err != nil {
		t.Fatal(err)
	}

	printGray("TestStatus:\n")

	printChangedFiles := func(entries map[string]ChangedFile) {
		untracked2Str := func(b bool) string {
			if b {
				return "Untracked"
			}
			return "Tracked  "
		}
		for k, v := range entries {
			fmt.Println("    "+untracked2Str(v.Untracked), WhatChangedToString(v.WhatChanged)+" "+k)
		}
	}

	testFailed := false

	// Sort our tests by the numbers in the folder names
	slices.SortFunc(tests, func(a, b os.DirEntry) int {
		num1, err := getNumberFromFolderName(a.Name())
		if err != nil {
			t.Fatal("Missing number prefix like \"25_\" in folder named:", a.Name())
		}
		num2, err := getNumberFromFolderName(b.Name())
		if err != nil {
			t.Fatal("Missing number prefix like \"25_\" in folder named:", b.Name())
		}

		return num1 - num2
	})
	for _, test := range tests {
		filesExtractPath := filepath.Join(testsPath, test.Name(), "files")
		defer os.RemoveAll(filesExtractPath)
		expectedPath := filepath.Join(testsPath, test.Name(), "expected.txt")
		if runtime.GOOS == "windows" {
			dontRunOnWindowsPath := filepath.Join(testsPath, test.Name(), "DO_NOT_RUN_ON_WINDOWS.txt")
			_, err := os.Stat(dontRunOnWindowsPath)
			if err == nil {
				fmt.Print("Test " + expectedPath + ": Not applicable to run on Windows")
				continue
			}

			expectedWindowsPath := filepath.Join(testsPath, test.Name(), "expected_windows.txt")
			_, err = os.Stat(expectedWindowsPath)
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

		expectedChangedFiles := make(map[string]ChangedFile)
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
			var untracked bool
			if split[0] == "Untracked" {
				untracked = true
			} else if split[0] == "Tracked" {
				untracked = false
			} else {
				t.Fatal("Invalid test, first word should be one of: \"Untracked\", \"Tracked\", case matching")
			}
			var whatChangedText string
			var pathText string
			if len(split) < 3 {
				whatChangedText = ""
				pathText = split[1]
			} else {
				whatChangedText = split[1]
				pathText = split[2]
			}

			pathText = filepath.FromSlash(pathText)

			expectedChangedFiles[pathText] = ChangedFile{WhatChanged: StringToWhatChanged(whatChangedText), Untracked: untracked}
		}

		_, err = os.Stat(filesExtractPath)
		if err != nil {
			zipFilePath := filepath.Join(testsPath, test.Name(), "files.zip")
			err := extractZipArchive(zipFilePath, filesExtractPath)
			if err != nil {
				t.Fatal(err)
			}
		}
		changedFiles, err := Status(filesExtractPath)

		// Let's also check for a crash when cancelling a StatusWithContext() call while we're at it.
		ctx, cancelFunc := context.WithCancel(context.Background())
		go StatusWithContext(ctx, filesExtractPath)
		cancelFunc()

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

	printEntries := func(entries map[string]GitIndexEntry) {
		for path, e := range entries {
			fmt.Println("    "+strconv.FormatUint(uint64(e.Mode), 8), hex.EncodeToString(e.Hash[:]), path)
		}
	}

	gitIndexEntriesMatch := func(a, b map[string]GitIndexEntry) bool {
		if len(a) != len(b) {
			return false
		}

		for k, v := range a {
			if !reflect.DeepEqual(b[k].Hash, v.Hash) {
				return false
			}

			// We don't test the mode
			/*if b[k].Mode != v.Mode {
				return false
			}*/
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

		// Sort our tests by the numbers in the folder names
		slices.SortFunc(versionTests, func(a, b os.DirEntry) int {
			num1, err := getNumberFromFolderName(a.Name())
			if err != nil {
				t.Fatal("Missing number prefix like \"25_\" in folder named:", a.Name())
			}
			num2, err := getNumberFromFolderName(b.Name())
			if err != nil {
				t.Fatal("Missing number prefix like \"25_\" in folder named:", b.Name())
			}

			return num1 - num2
		})
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

			expectedEntries := make(map[string]GitIndexEntry)
			var expectedError error = nil

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "Error text:") {
					expectedEntries = make(map[string]GitIndexEntry)
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

				expectedEntries[pathName] = GitIndexEntry{Hash: [20]byte(sha1HashBytes)}
			}
			file.Close()

			ctx := context.WithoutCancel(context.Background())
			entries, err := ParseGitIndex(ctx, indexPath)
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

func TestBenchmarkParseGitIndex(t *testing.T) {
	howManyTimes := 10
	fmt.Print("[Benchmark] Calling ParseGitIndex() on Linux .git/index ", howManyTimes, " times:")

	indexPath := "benchmark_indexes/torvalds_linux"
	expectedEntriesLength := 92192

	start := time.Now()

	for i := 0; i < howManyTimes; i++ {
		ctx := context.WithoutCancel(context.Background())
		entries, err := ParseGitIndex(ctx, indexPath)
		if err != nil {
			t.Fatal("Got an error while benchmarking ParseGitIndex(): ", err)
		}

		if len(entries) != expectedEntriesLength {
			t.Fatal("Expected ", expectedEntriesLength, " entries, but got: ", len(entries))
		}
	}

	duration := time.Since(start)
	fmt.Println(" " + duration.String())
}

func TestIncludingDirectories(t *testing.T) {
	c := func(path string) string {
		return filepath.FromSlash(path)
	}

	changedFiles := map[string]ChangedFile{
		c("main.go"):        {},
		c("screenshots/hi"): {},
	}

	expected := map[string]ChangedFile{
		c("main.go"):        {},
		c("screenshots/hi"): {},
		c("screenshots"):    {},
	}

	got := IncludingDirectories(changedFiles)

	if !reflect.DeepEqual(got, expected) {
		t.Fatal("TestIncludingDirectories (1) got an unexpected value")
	}

	changedFiles = map[string]ChangedFile{
		c("main.go"):                        {},
		c("screenshots/hi"):                 {},
		c("folder/anotherfolder/hello.txt"): {},
		c("folder/anotherfolder/hi"):        {},
		c("folder/file.txt"):                {},
		c("folder/anotherfile.txt"):         {},
	}

	expected = map[string]ChangedFile{
		c("main.go"):                        {},
		c("screenshots/hi"):                 {},
		c("folder/anotherfolder/hello.txt"): {},
		c("folder/anotherfolder/hi"):        {},
		c("folder/file.txt"):                {},
		c("folder/anotherfile.txt"):         {},

		c("screenshots"):          {},
		c("folder/anotherfolder"): {},
		c("folder"):               {},
	}

	got = IncludingDirectories(changedFiles)

	if !reflect.DeepEqual(got, expected) {
		t.Fatal("TestIncludingDirectories (2) got an unexpected value")
	}

	empty := map[string]ChangedFile{}
	got = IncludingDirectories(empty)
	if !reflect.DeepEqual(got, empty) {
		t.Fatal("TestIncludingDirectories expected no values, but got some")
	}

	changedFiles = map[string]ChangedFile{
		c("many/folders/for/sure/oh/yeah.txt"): {},
	}
	expected = map[string]ChangedFile{
		c("many"):                              {},
		c("many/folders"):                      {},
		c("many/folders/for"):                  {},
		c("many/folders/for/sure"):             {},
		c("many/folders/for/sure/oh"):          {},
		c("many/folders/for/sure/oh/yeah.txt"): {},
	}
	got = IncludingDirectories(changedFiles)
	if !reflect.DeepEqual(got, expected) {
		t.Fatal("TestIncludingDirectories did not recursively add directories to the map")
	}

	changedFiles = map[string]ChangedFile{
		c("main.go"):        {},
		c("screenshots/hi"): {},
	}
	expected = map[string]ChangedFile{
		c("main.go"):        {},
		c("screenshots/hi"): {},
	}
	IncludingDirectories(changedFiles)
	if !reflect.DeepEqual(changedFiles, expected) {
		t.Fatal("TestIncludingDirectories overwrote the input argument!")
	}
}

func TestExcludingDirectories(t *testing.T) {
	c := func(path string) string {
		return filepath.FromSlash(path)
	}

	changedFiles := map[string]ChangedFile{
		c("main.go"):        {},
		c("screenshots/hi"): {},
	}
	including := IncludingDirectories(changedFiles)
	excluding := ExcludingDirectories(including)
	if !reflect.DeepEqual(changedFiles, excluding) {
		t.Fatal("TestExcludingDirectories (1) got an unexpected value")
	}

	changedFiles = map[string]ChangedFile{
		c("main.go"):        {},
		c("screenshots/hi"): {},
		c("screenshots"):    {},
	}
	expected := map[string]ChangedFile{
		c("main.go"):        {},
		c("screenshots/hi"): {},
	}
	got := ExcludingDirectories(changedFiles)
	if !reflect.DeepEqual(got, expected) {
		t.Fatal("TestExcludingDirectories (2) got an unexpected value")
	}

	changedFiles = map[string]ChangedFile{
		c("main.go"):        {},
		c("screenshots/hi"): {},
		c("screenshots"):    {},
	}
	expected = map[string]ChangedFile{
		c("main.go"):        {},
		c("screenshots/hi"): {},
		c("screenshots"):    {},
	}
	ExcludingDirectories(changedFiles)
	if !reflect.DeepEqual(changedFiles, expected) {
		t.Fatal("TestExcludingDirectories overwrote the input argument!")
	}

	changedFiles = map[string]ChangedFile{
		c("many"):                              {},
		c("many/folders"):                      {},
		c("many/folders/for"):                  {},
		c("many/folders/for/sure"):             {},
		c("many/folders/for/sure/oh"):          {},
		c("many/folders/for/sure/oh/yeah.txt"): {},
	}
	expected = map[string]ChangedFile{
		c("many/folders/for/sure/oh/yeah.txt"): {},
	}
	got = ExcludingDirectories(changedFiles)
	if !reflect.DeepEqual(got, expected) {
		t.Fatal("TestExcludingDirectories did not recursively add directories to the map")
	}
}

func TestParseGitIndexFromMemoryUntrustedAllocationCount(t *testing.T) {
	// Only the 12-byte header, with a large amount of entries (1827392984) specified in the last 4 bytes.
	// Previously, space for these entries would be allocated no questions asked,
	//  resulting in a runtime out of memory panic.
	data := []byte("DIRC\x00\x00\x00\x02l\xeb\xcd\xd8")

	ctx := context.WithoutCancel(context.Background())
	// However, we now have an optional max amount of entries to pre-allocate. (1000 in this case)
	_, err := ParseGitIndexFromMemory(ctx, data, 1000)

	if err == nil {
		t.Fatal("Expected an error, but got nil")
	}
}

// Fuzz for crashes in ParseGitIndexFromMemory()
func FuzzParseGitIndexFromMemory(f *testing.F) {
	files, err := os.ReadDir("fuzz_indexes")
	if err != nil {
		f.Fatal("Failed to open fuzz_indexes:", err)
	}

	for _, file := range files {
		data, err := os.ReadFile("fuzz_indexes" + string(os.PathSeparator) + file.Name())
		if err != nil {
			f.Fatal("Failed to read file:", err)
		}

		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		ctx := context.WithoutCancel(context.Background())
		_, _ = ParseGitIndexFromMemory(ctx, data, 1000)
	})
}
