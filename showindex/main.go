package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"strconv"

	"github.com/kivattt/gogitstatus"
)

func main() {
	args := os.Args

	if len(args) < 2 {
		fmt.Println("Usage: showindex [git index file]")
		os.Exit(0)
	}

	path := os.Args[1]

	ctx := context.WithoutCancel(context.Background())
	entries, err := gogitstatus.ParseGitIndex(ctx, path)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	for path, e := range entries {
		fmt.Println(strconv.FormatInt(int64(e.Mode&gogitstatus.OBJECT_TYPE_MASK>>13), 2)+strconv.FormatInt(int64(e.Mode&uint32(fs.ModePerm)), 8), hex.EncodeToString(e.Hash[:]), path)
	}
}
