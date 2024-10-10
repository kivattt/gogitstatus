package gogitstatus

import (
	"fmt"
	"testing"
)

func TestStatus(t *testing.T) {
	entries, err := Status(".")
	fmt.Print("\n\n\n")
	for _, e := range entries {
		fmt.Println(e)
	}
	fmt.Println("Error:", err)
}
