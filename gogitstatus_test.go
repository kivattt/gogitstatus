package gogitstatus

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseGitIndex(t *testing.T) {
	testsPath := "./tests-index-parser"
	tests, err := os.ReadDir(testsPath)
	if err != nil {
		t.Fatal(err)
	}

	printEntries := func(entries []GitIndexEntry) {
		for _, e := range entries {
			fmt.Println("    " + hex.EncodeToString(e.hash), e.path)
		}
	}

	gitIndexEntriesMatch := func(a, b []GitIndexEntry) bool {
		if len(a) != len(b) {
			return false
		}

		for i := 0; i < len(a); i++ {
			left, right := a[i], b[i]
			if !reflect.DeepEqual(left.hash, right.hash) {
				return false
			}

			if left.path != right.path {
				return false
			}
		}
		return true
	}

	printRed := func(text string) {
		fmt.Print("\x1b[31m" + text + "\x1b[0m")
	}

	printGreen := func(text string) {
		fmt.Print("\x1b[32m" + text + "\x1b[0m")
	}

	for _, version := range tests {
		versionTests, err := os.ReadDir(filepath.Join(testsPath, version.Name()))
		if err != nil {
			t.Fatal(err)
		}

		for _, versionTest := range versionTests {
			indexPath := filepath.Join(testsPath, version.Name(), versionTest.Name(), "index")
			expectedPath := filepath.Join(testsPath, version.Name(), versionTest.Name(), "expected.txt")
			inTest := "In test file " + expectedPath + ": "
			fmt.Print("Test file " + expectedPath + ": ")

			file, err := os.Open(expectedPath)
			if err != nil {
				printRed("Failed\n")
				t.Fatal(inTest, err)
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
					t.Fatal(inTest, err)
				}

				expectedEntries = append(expectedEntries, GitIndexEntry{path: pathName, hash: sha1HashBytes})
			}
			file.Close()

			entries, err := ParseGitIndex(indexPath)
			if expectedError == nil && err != nil {
				printRed("Failed\n")
				t.Fatal(inTest, "expected no error, but got: " + err.Error())
			}

			if err != nil && expectedError != nil {
				if err.Error() != expectedError.Error() {
					printRed("Failed\n")
					t.Fatal(inTest, "expected error text \"" + expectedError.Error() + "\", but got: \"" + err.Error() + "\"")
				}
			}

			if !gitIndexEntriesMatch(entries, expectedEntries) {
				printRed("Failed\n")

				fmt.Println("Expected entries:")
				printEntries(expectedEntries)
				fmt.Println("But got:")
				printEntries(entries)
				t.Fatal(inTest, "See above ^")
			}

			printGreen("Success\n")
		}
	}
}

/*func TestStatus(t *testing.T) {
	entries, err := Status(".")
	if err != nil {
		fmt.Println("error: " + err.Error())
		return
	}
	for _, e := range entries {
		fmt.Println("\x1b[0;31m" + e + "\x1b[0m")
	}
}*/
