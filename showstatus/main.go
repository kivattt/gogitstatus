package main

import (
	"fmt"
	"os"

	"github.com/kivattt/gogitstatus"
	//	"github.com/pkg/profile"
)

func main() {
	//	defer profile.Start(profile.MemProfile, profile.MemProfileRate(1)).Stop()

	args := os.Args

	path, _ := os.Getwd()
	if len(args) > 1 {
		path = args[1]
	}

	paths, err := gogitstatus.Status(path)

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
	for k, e := range unstaged {
		whatChangedStr := ""
		if e.WhatChanged&gogitstatus.DELETED != 0 {
			whatChangedStr = "deleted:  "
		} else if e.WhatChanged&gogitstatus.DATA_CHANGED != 0 {
			whatChangedStr = "modified: "
		}

		//whatChangedStr := gogitstatus.WhatChangedToString(e.WhatChanged)
		if whatChangedStr != "" {
			whatChangedStr += " "
		}
		fmt.Println("        \x1b[0;31m" + whatChangedStr + k + "\x1b[0m")
	}

	if len(untracked) > 0 {
		fmt.Println("Untracked files:")
	}
	for k, e := range untracked {
		whatChangedStr := ""
		if e.WhatChanged&gogitstatus.DELETED != 0 {
			whatChangedStr = "deleted:  "
		} else if e.WhatChanged&gogitstatus.DATA_CHANGED != 0 {
			whatChangedStr = "modified: "
		}

		//whatChangedStr := gogitstatus.WhatChangedToString(e.WhatChanged)
		if whatChangedStr != "" {
			whatChangedStr += " "
		}
		fmt.Println("        \x1b[0;31m" + whatChangedStr + k + "\x1b[0m")
	}
}
