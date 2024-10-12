package main

import (
	"encoding/hex"
	"fmt"
	"gogitstatus"
	"os"
	"strconv"
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
		fmt.Println("error:", err)
		os.Exit(1)
	}

	for _, e := range entries {
		fmt.Println(strconv.FormatInt(int64(e.Mode&0b111111111), 8), hex.EncodeToString(e.Hash), e.Path)
	}
}
