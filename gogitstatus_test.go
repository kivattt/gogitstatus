package gogitstatus

import (
	"fmt"
	"testing"
)

func TestStatus(t *testing.T) {
	entries, _ := Status(".")
	for _, e := range entries {
		fmt.Println("\x1b[0;31m" + e + "\x1b[0m")
	}
}
