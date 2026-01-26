package gogitstatus

import (
	"path/filepath"
	"testing"
)

func TestMyDir(t *testing.T) {
	type TestCase struct {
		path     string
		expected string
	}

	tests := []TestCase{
		{"", "."},
		{"/", "/"},
		{"my/", "my"},
		{"my/folder", "my"},
		{"my/folder/", "my/folder"},
	}
	for _, test := range tests {
		result := myDir(test.path)
		if result != filepath.FromSlash(test.expected) {
			t.Fatal("Expected: \"" + test.expected + "\" but got: \"" + result + "\"")
		}
	}
}
