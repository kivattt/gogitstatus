package gogitstatus

import (
	"fmt"
	"testing"
)

func TestStatus(t *testing.T) {
	entries, err := Status(".")
	for _, e := range entries {
		fmt.Println(OperationToString(e.operation), e.entry.Name())
	}
	fmt.Println("Error:", err)
}
