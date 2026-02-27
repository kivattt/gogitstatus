package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kivattt/gogitstatus"
	"golang.org/x/term"
	//	"github.com/pkg/profile"
)

func usage(programName string) {
	fmt.Println("Usage: " + programName + " [OPTIONS] <optional path>")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("\t-h, --help")
	fmt.Println("\t--verbose")
	fmt.Println("\t--timeout=milliseconds")
}

func main() {
	//defer profile.Start(profile.CPUProfile).Stop()

	useColor := true
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		useColor = false // Output is piped, don't colorize the output
	}

	if runtime.GOOS == "windows" {
		useColor = false
	}

	args := os.Args

	path, _ := os.Getwd()
	help := false
	verbose := false
	timeoutMillis := -1

	// I hate the flag package, this is better
	for i := 1; i < len(args); i++ {
		if args[i] == "-h" || args[i] == "--help" {
			help = true
		} else if args[i] == "--verbose" {
			verbose = true
		} else if strings.HasPrefix(args[i], "--timeout=") {
			milliseconds, err := strconv.Atoi(args[i][len("--timeout="):])
			if err != nil {
				usage(args[0])
				os.Exit(1)
			}

			if milliseconds < 0 {
				fmt.Println("timeout can not be negative")
				os.Exit(1)
			}

			timeoutMillis = milliseconds
		} else {
			// Last arg that isn't "--verbose" is the path
			path = args[i]
		}
	}

	if help {
		usage(args[0])
		os.Exit(0)
	}

	var paths map[string]gogitstatus.ChangedFile
	var err error
	var wg sync.WaitGroup
	wg.Add(1)

	ctx, cancelFunc := context.WithCancel(context.Background())
	go func() {
		paths, err = gogitstatus.StatusWithContext(ctx, path)
		wg.Done()
	}()

	if timeoutMillis != -1 {
		go func() {
			time.Sleep(time.Duration(timeoutMillis) * time.Millisecond)
			fmt.Println("Cancelling!")
			cancelFunc()
		}()
	}

	wg.Wait()

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

		whatChangedStr = gogitstatus.WhatChangedToString(elem.WhatChanged)
		if whatChangedStr != "" {
			panic("Somehow there's a whatchanged on an untracked file...")
			whatChangedStr += " "
		}

		if useColor {
			fmt.Println("        \x1b[0;31m" + whatChangedStr + key + "\x1b[0m")
		} else {
			fmt.Println("        " + whatChangedStr + key)
		}
	}
}
