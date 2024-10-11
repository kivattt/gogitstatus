package main

import (
	"encoding/hex"
	"fmt"
	"gogitstatus"
	"os"
)

func main() {
	args := os.Args

	if len(args) < 2 {
		fmt.Println("Usage: showindex [git index file]")
		os.Exit(0)
	}

	path := os.Args[1]

	entries, err := gogitstatus.ParseGitIndex(path)
	if err != nil {
		fmt.Println("error: " + err.Error())
		return
	}

	for _, e := range entries {
		fmt.Println(hex.EncodeToString(e.Hash), e.Path)
	}
}
