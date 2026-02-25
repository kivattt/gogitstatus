package gogitstatus

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	//"github.com/sabhiram/go-gitignore"
	ignore "github.com/botondmester/goignore"
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
				fmt.Println("Test " + filepath.Dir(expectedPath) + ": Not applicable to run on Windows")
				continue
			}

			expectedWindowsPath := filepath.Join(testsPath, test.Name(), "expected_windows.txt")
			_, err = os.Stat(expectedWindowsPath)
			if err == nil {
				expectedPath = expectedWindowsPath
			}
		}
		fmt.Print("Test " + filepath.Dir(expectedPath) + ": ")

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

		seeIfFailing := func(changedFiles map[string]ChangedFile, err error,
			expectedAnyError bool, expectedError error, expectedChangedFiles map[string]ChangedFile,
			outputToStdout bool) bool {
			if expectedAnyError && err == nil {
				if outputToStdout {
					fmt.Println("expected any error, but got nil")
				}
				return true
			}

			if !expectedAnyError && expectedError == nil && err != nil {
				if outputToStdout {
					printRed("Failed\n")
					fmt.Println("expected no error, but got: " + err.Error())
				}
				return true
			}

			if !expectedAnyError && err != nil && expectedError != nil {
				if err.Error() != expectedError.Error() {
					if outputToStdout {
						printRed("Failed\n")
						fmt.Println("expected error text \"" + expectedError.Error() + "\", but got: \"" + err.Error() + "\"")
					}
					return true
				}
			}

			if !(len(changedFiles) == 0 && len(expectedChangedFiles) == 0) && !reflect.DeepEqual(changedFiles, expectedChangedFiles) {
				if outputToStdout {
					printRed("Failed\n")

					fmt.Println("Expected entries:")
					printChangedFiles(expectedChangedFiles)
					fmt.Println("But got:")
					printChangedFiles(changedFiles)
				}
				return true
			}

			return false
		}

		singleThreadFailed := false
		multiThreadFailed := false

		// Used to make some multi-threaded functions more predictable for testing. (I had a skipDir() bug which required this)
		numCPUs := 1
		changedFilesSerial, errSerial := Status(filesExtractPath, numCPUs)
		failed := seeIfFailing(changedFilesSerial, errSerial, expectedAnyError, expectedError, expectedChangedFiles, true)
		if failed {
			singleThreadFailed = true
			testFailed = true
		}

		// Multi-threaded:
		// Atleast 2 goroutines, used to test multithreaded / parallel functions
		numCPUs = max(2, runtime.NumCPU())
		changedFiles, err := Status(filesExtractPath, numCPUs)
		failed = seeIfFailing(changedFiles, err, expectedAnyError, expectedError, expectedChangedFiles, !singleThreadFailed) // If the single-threaded test failed, don't output anything from this multi-threaded test
		if failed {
			multiThreadFailed = true
			testFailed = true
		}

		// Special yellow warnings when a test failed only in single or multi-threaded, but not in the other
		if multiThreadFailed && (!singleThreadFailed) {
			fmt.Println("\x1b[33m^ Failed only when running multi-threaded (" + strconv.Itoa(numCPUs) + " CPUs) (Bug in goignore which would otherwise be masked by our skipping of directories? Or just a threading bug?)\x1b[0m")
		} else if singleThreadFailed && (!multiThreadFailed) {
			fmt.Println("\x1b[33m^ Failed only when running single-threaded !")
			fmt.Println("  May be a bug in our skipdir logic? Try setting gogitstatus_debug_disable_skipdir = true.")
			fmt.Println("  If that fixes it, it's probably that.\x1b[0m")
		}

		if singleThreadFailed || multiThreadFailed {
			continue
		}

		// Let's also check for a crash when cancelling a StatusWithContext() call while we're at it.
		ctx, cancelFunc := context.WithCancel(context.Background())
		go StatusWithContext(ctx, filesExtractPath)
		cancelFunc()

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
			fmt.Print("Test " + filepath.Dir(expectedPath) + ": ")

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

	fmt.Println()
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
	files, err := os.ReadDir("test-data" + string(os.PathSeparator) + "fuzz_indexes")
	if err != nil {
		f.Fatal("Failed to open test-data"+string(os.PathSeparator)+"fuzz_indexes:", err)
	}

	for _, file := range files {
		data, err := os.ReadFile("test-data" + string(os.PathSeparator) + "fuzz_indexes" + string(os.PathSeparator) + file.Name())
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

// This test copied from my file manager
// See: https://github.com/kivattt/fen/blob/main/util_test.go#L365
func TestSpreadArrayIntoSlicesForGoroutines(t *testing.T) {
	type TestCase struct {
		arrayLength   int
		numGoroutines int
		expected      []Slice
	}

	tests := []TestCase{
		{0, 0, []Slice{}},
		{1, 4, []Slice{{0, 1}}}, // Less elements than goroutines, will use arrayLength goroutines instead.
		{1, 1, []Slice{{0, 1}}},
		{2, 2, []Slice{{0, 1}, {1, 1}}},
		{3, 2, []Slice{{0, 1}, {1, 2}}},
		{3, 4, []Slice{{0, 1}, {1, 1}, {2, 1}}}, // Less elements than goroutines, will use arrayLength goroutines instead.
		{100, 2, []Slice{{0, 50}, {50, 50}}},
		{500, 4, []Slice{{0, 125}, {125, 125}, {250, 125}, {375, 125}}},
		{501, 4, []Slice{{0, 125}, {125, 125}, {250, 125}, {375, 126}}},
		{504, 4, []Slice{{0, 126}, {126, 126}, {252, 126}, {378, 126}}},
		{505, 4, []Slice{{0, 126}, {126, 126}, {252, 126}, {378, 127}}},
	}

	for _, test := range tests {
		got := SpreadArrayIntoSlicesForGoroutines(test.arrayLength, test.numGoroutines)

		if !reflect.DeepEqual(got, test.expected) {
			t.Fatal("Expected", test.expected, "but got:", got)
		}
	}
}

func TestSkipDir(t *testing.T) {
	printGray("TestSkipDir:")
	failed := false
	defer func() {
		if failed {
			printRed(" Failed\n")
		} else {
			printGreen(" Success\n")
		}
	}()
	type TestCase struct {
		paths    []string
		index    int
		expected int // -1 meaning an error
	}

	tests := []TestCase{
		{[]string{"folder/file.txt", "folder/file2.txt", "folder/file3.txt", "folder2/file.txt"}, 0, 3},
		{[]string{"/", "/home/", "/home/file.txt", "/home/file2.txt", "/sauce/", "/sauce/file.txt"}, 0, -1}, // The parent folder is root, skips everything (-1)
		{[]string{"/home/", "/home/file.txt", "/home/file2.txt", "/sauce/", "/sauce/file.txt"}, 0, 3},
		{[]string{"/file.txt", "/file2.txt"}, 0, -1}, // The parent folder is root, skips everything (-1)
		{[]string{"/folder/file.txt", "/folder/file2.txt", "/folder2/file.txt", "/folder2/file2.txt"}, 0, 2},
		{[]string{"/folder/file.txt", "/folder/file2.txt", "/folder2/file.txt", "/folder2/file2.txt"}, 2, -1}, // Reached the end (-1)
		{[]string{"hi", "hi"}, 1, -1},     // The parent folder is root ".", skips everything (-1)
		{[]string{"."}, 0, -1},            // The parent folder is root ".", skips everything (-1)
		{[]string{".", "./hello"}, 0, -1}, // The parent folder is root ".", skips everything (-1)
		{[]string{"2_folder/folder/", "2_folder/folder/file.txt", "5", "4"}, 0, 2},
		{[]string{"2_folder/folder/", "2_folder/folder/file.txt", "5", "4"}, 1, 2},
		{[]string{"2_folder/folder/", "2_folder/folder/file.txt", "5", "4"}, 3, -1},
		{[]string{"ignored_folder/file.txt", "file.txt"}, 0, 1},

		{[]string{"folder/", "folder/hi"}, 0, -1},
		{[]string{"folder/", "folder/hi/"}, 0, -1},         // A child folder is not supposed to be the next folder.
		{[]string{"folder/file.txt", "folder/hi/"}, 0, -1}, // A child folder is not supposed to be the next folder.
	}

	for i, test := range tests {
		result, err := skipDir(test.paths, test.index)

		if err == nil {
			if result != test.expected {
				failed = true
				t.Fatal("Expected", test.expected, "in test index", i, "but got:", result)
			}
		} else {
			if test.expected != -1 {
				failed = true
				t.Fatal("Expected no error in test index", i, ", but got:", err.Error())
			}
		}
	}
}

// This test checks if we forgot to decrement i by 1 after i = skipDir()
func TestUntrackedPathsNotIgnoredWorker(t *testing.T) {
	printGray("TestUntrackedPathsNotIgnoredWorker:")
	failed := false
	defer func() {
		if failed {
			printRed(" Failed\n")
		} else {
			printGreen(" Success\n")
		}
	}()
	ignoreObj := ignore.CompileIgnoreLines("ignored_folder/")
	ignoresCache := map[string]*ignore.GitIgnore{".": ignoreObj}
	indexEntries := make(map[string]GitIndexEntry) // Empty

	// Doesn't spawn any goroutines, just runs them sequentially
	runNumGoroutines := func(paths []string, num int) map[string]ChangedFile {
		results := make([]map[string]ChangedFile, num)

		slices := SpreadArrayIntoSlicesForGoroutines(len(paths), num)
		for threadIdx, slice := range slices {
			ourSlice := paths[slice.start : slice.start+slice.length]
			results[threadIdx] = untrackedPathsNotIgnoredWorker(ourSlice, ignoresCache, indexEntries, true)
		}

		// Merge the results
		for i := 1; i < len(results); i++ {
			for k, v := range results[i] {
				results[0][k] = v
			}
		}

		return results[0]
	}

	// Test for missing decrement of i variable
	paths := []string{"ignored_folder/", "file.txt"}
	expected := map[string]ChangedFile{"file.txt": {Untracked: true}}
	result := runNumGoroutines(paths, 1)
	if !maps.Equal(result, expected) {
		failed = true
		t.Fatal("Expected:", expected, "but got:", result)
	}
}

func TestConvertCRLFToLF(t *testing.T) {
	printGray("TestConvertCRLFToLF:")
	failed := false
	defer func() {
		if failed {
			printRed(" Failed\n")
		} else {
			printGreen(" Success\n")
		}
	}()

	type TestCase struct {
		input    string
		expected string
	}

	tests := []TestCase{
		{"", ""},
		{"\r", ""},
		{"\r\n", "\n"},
		{"line 1\r\nline 2\r\nline 3\r\n", "line 1\nline 2\nline 3\n"},
	}

	for _, test := range tests {
		result := convertCRLFToLF([]byte(test.input))
		if !reflect.DeepEqual(result, []byte(test.expected)) {
			fmt.Println("Expected", []byte(test.expected), "but got:", result)
			failed = true
		}
	}
}

func TestBenchmarkConvertCRLFToLF(t *testing.T) {
	fmt.Println()

	entries, err := os.ReadDir(filepath.Join("test-data", "benchmark_crlf_conversion"))
	if err != nil {
		panic("Failed to read files in test-data" + string(os.PathSeparator) + "benchmark_crlf_conversion")
	}

	totalBytes := 0
	fileData := make([][]byte, 0)
	for _, entry := range entries {
		path := filepath.Join("test-data", "benchmark_crlf_conversion", entry.Name())

		file, err := os.Open(path)
		if err != nil {
			panic("Failed to open file: " + path)
		}

		stat, err := os.Lstat(path)
		if err != nil {
			panic("Failed to stat file: " + path)
		}

		totalBytes += int(stat.Size())

		data, err := openFileData(file, stat)
		fileData = append(fileData, data)
	}

	/* Average case, using files from test-data/benchmark_crlf_conversion */
	nTimes := 1000
	printGray("[Benchmark] ConvertCRLFToLF() " + strconv.Itoa(nTimes) + " times on " + strconv.Itoa(totalBytes/1000) + " kB: ")
	start := time.Now()
	for i := 0; i < nTimes; i++ {
		for _, data := range fileData {
			convertCRLFToLF(data)
		}
	}
	fmt.Println(time.Since(start))

	/* Worst-case for bytes.IndexByte() */
	nTimesWorstCase := 1000
	printGray("[Benchmark] ConvertCRLFToLF() " + strconv.Itoa(nTimesWorstCase) + " times on 100 kB, worst-case for bytes.IndexByte(): ")

	data := bytes.Repeat([]byte{'\r'}, 100000) // 100 kB

	start = time.Now()
	for i := 0; i < nTimesWorstCase; i++ {
		convertCRLFToLF(data)
	}
	fmt.Println(time.Since(start))
}

func TestBenchmarkParseGitIndex(t *testing.T) {
	howManyTimes := 10
	printGray("[Benchmark] ParseGitIndex() on Linux .git/index " + strconv.Itoa(howManyTimes) + " times:")

	indexPath := filepath.Join("test-data", "benchmark_indexes", "torvalds_linux")
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
