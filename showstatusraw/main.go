package main

import (
	"fmt"
	"gogitstatus"
	"os"
	//	"runtime/pprof"
)

func main() {
	/*	f, _ := os.Create("profile.prof")
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()*/

	args := os.Args
	if len(args) < 3 {
		fmt.Println("Usage: showstatusraw [directory] [git index file]")
		fmt.Println("Example: showstatusraw files index")
		os.Exit(0)
	}

	path := args[1]
	indexPath := args[2]

	paths, err := gogitstatus.StatusRaw(path, indexPath, true)

	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	untracked2Str := func(b bool) string {
		if b {
			return "Untracked"
		}
		return "Tracked  "
	}

	for k, e := range paths {
		whatChangedStr := gogitstatus.WhatChangedToString(e.WhatChanged)
		if whatChangedStr != "" {
			whatChangedStr += " "
		}
		fmt.Println(untracked2Str(e.Untracked), whatChangedStr+k)
	}
}
