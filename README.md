## gogitstatus

[![Go Reference](https://pkg.go.dev/badge/github.com/kivattt/gogitstatus.svg)](https://pkg.go.dev/github.com/kivattt/gogitstatus)
[![Go Report Card](https://goreportcard.com/badge/github.com/kivattt/gogitstatus)](https://goreportcard.com/report/github.com/kivattt/gogitstatus)

gogitstatus is a library for finding unstaged/untracked files in local Git repositories\
Tested for Linux, FreeBSD and Windows\
This library is used in my terminal file manager [fen](https://github.com/kivattt/fen)

To try out `gogitstatus.Status()`, run the showstatus program:
```console
cd showstatus
go build
./showstatus . # In any git repository
```

To try out `gogitstatus.ParseIndex()`, run the showindex program:
```console
cd showindex
go build
./showindex ../.git/index
```

## Running tests
Run `go test -race` to run all the tests.

Run `go test -fuzz=FuzzParseGitIndexFromMemory` to fuzz for crashes in the `ParseGitIndexFromMemory()` function.

If you are developing on Linux, you can run the `./run_windows_test.sh` script to test on "Windows" with [wine](https://www.winehq.org/)

<details>
<summary>Check for context cancellation latency issues</summary>

`StatusWithContext()` is a cancellable function.\
To check if we forgot a `select` block to handle cancelling somewhere, you can run this tool to graph the timeout and the actual time spent.\
Ideally, both the timeout and time spent should be linear and close to eachother.
This tool generates a .csv file you can load into Excel or Libreoffice to graph the data.
```
cd showstatus
go build
./graph_timeout_delays.sh /path/to/large/repository 2> output.csv
```

### Example of a cancellation bug
![Example of a latency issue because we forgot to handle cancellation in a loop. Rendered with Liberoffice](img/bad_context_cancel.png)
</details>

## Git Index file format resources
https://git-scm.com/docs/index-format (missing some visual separation...)\
https://github.com/git/git/blob/master/read-cache.c

## TODO
- Deal with .git files that point the real .git folder elsewhere (submodules or something)
- Support exclude file priority (like core.excludesFile in config and other XDG\_CONFIG stuff)
- Support SHA-256
- Support other Git Index versions besides 2
- Deal with .gitattributes (and XDG\_CONFIG stuff) to determine whether we need to hash with line endings normalized. See: `tests-status/36_line_ending_conversion_during_hash/README.md`
