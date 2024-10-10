package gogitstatus

import (
	"fmt"
	"testing"
)

func TestStatus(t *testing.T) {
	entries, _ := Status(".")
	for _, e := range entries {
		fmt.Println(e)
	}
}
