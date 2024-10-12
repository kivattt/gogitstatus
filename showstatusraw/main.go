package main

import (
	"fmt"
	"gogitstatus"
	"os"
)

func main() {
	args := os.Args
	if len(args) < 3 {
		fmt.Println("Usage: showstatusraw [directory] [git index file]")
		fmt.Println("Example: showstatusraw files index")
		os.Exit(0)
	}

	path := args[1]
	indexPath := args[2]

	paths, err := gogitstatus.StatusRaw(path, indexPath)

	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	for _, e := range paths {
		fmt.Println(e)
	}
}
