package main

import (
	"fmt"
	"gogitstatus"
	"os"
	"path/filepath"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		fmt.Println("Usage: showstatus [local git repository]")
		os.Exit(0)
	}

	path := args[1]

	dotGit := ".git"
	dotGitPath := filepath.Join(path, dotGit)
	_, err := os.Stat(dotGitPath)
	if err != nil {
		dotGit = ".gitfake" // Since we can't include a git repo inside the gogitstatus repo, its .git is named .gitfake
	}

	paths, err := gogitstatus.Status(path, dotGit)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	for _, e := range paths {
		fmt.Println(e)
	}
}
