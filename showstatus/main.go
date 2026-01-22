package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/kivattt/gogitstatus"
	"golang.org/x/term"
	//	"github.com/pkg/profile"
)

func main() {
	//defer profile.Start(profile.CPUProfile).Stop()

	useColor := true
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		useColor = false // Output is piped, don't colorize the output
	}

	args := os.Args

	path, _ := os.Getwd()
	verbose := false

	// I hate the flag package, this is better
	for i := 1; i < len(args); i++ {
		if args[i] == "--verbose" {
			verbose = true
		} else {
			// Last arg that isn't "--verbose" is the path
			path = args[i]
		}
	}

	paths, err := gogitstatus.Status(path)

	// Removes deleted paths
	//paths = gogitstatus.ExcludingDeleted(paths)

	// These should cancel out eachother:
	//paths = gogitstatus.IncludingDirectories(paths)
	//paths = gogitstatus.ExcludingDirectories(paths)

	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	unstaged := make(map[string]gogitstatus.ChangedFile)
	untracked := make(map[string]gogitstatus.ChangedFile)

	for k, e := range paths {
		if e.Untracked {
			untracked[k] = e
		} else {
			unstaged[k] = e
		}
	}

	if len(unstaged) > 0 {
		fmt.Println("Changes not staged for commit:")
	}

	unstagedKeysSorted := make([]string, len(unstaged))
	i := 0
	for key := range unstaged {
		unstagedKeysSorted[i] = key
		i++
	}
	sort.Strings(unstagedKeysSorted)
	for _, key := range unstagedKeysSorted {
		elem := unstaged[key]
		whatChangedStr := ""
		if elem.WhatChanged&gogitstatus.DELETED != 0 {
			whatChangedStr = "deleted:   "
		} else if elem.WhatChanged&gogitstatus.DATA_CHANGED != 0 || elem.WhatChanged&gogitstatus.MODE_CHANGED != 0 {
			whatChangedStr = "modified:  "
		}

		if verbose {
			whatChangedStr = gogitstatus.WhatChangedToString(elem.WhatChanged)
		}
		if whatChangedStr != "" {
			whatChangedStr += " "
		}

		if useColor {
			fmt.Println("        \x1b[0;31m" + whatChangedStr + key + "\x1b[0m")
		} else {
			fmt.Println("        " + whatChangedStr + key)
		}
	}

	if len(untracked) > 0 {
		fmt.Println("Untracked files:")
	}

	untrackedKeysSorted := make([]string, len(untracked))
	i = 0
	for key := range untracked {
		untrackedKeysSorted[i] = key
		i++
	}
	sort.Strings(untrackedKeysSorted)
	for _, key := range untrackedKeysSorted {
		elem := untracked[key]
		whatChangedStr := ""
		if elem.WhatChanged&gogitstatus.DELETED != 0 {
			whatChangedStr = "deleted:   "
		} else if elem.WhatChanged&gogitstatus.DATA_CHANGED != 0 || elem.WhatChanged&gogitstatus.MODE_CHANGED != 0 {
			whatChangedStr = "modified:  "
		}

		if verbose {
			whatChangedStr = gogitstatus.WhatChangedToString(elem.WhatChanged)
		}
		if whatChangedStr != "" {
			whatChangedStr += " "
		}

		if useColor {
			fmt.Println("        \x1b[0;31m" + whatChangedStr + key + "\x1b[0m")
		} else {
			fmt.Println("        " + whatChangedStr + key)
		}
	}
}
