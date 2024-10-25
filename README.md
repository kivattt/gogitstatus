## This library is still in development, and unfit for use

[![Go Reference](https://pkg.go.dev/badge/github.com/kivattt/gogitstatus.svg)](https://pkg.go.dev/github.com/kivattt/gogitstatus)
[![Go Report Card](https://goreportcard.com/badge/github.com/kivattt/gogitstatus)](https://goreportcard.com/report/github.com/kivattt/gogitstatus)

This is a library for finding unstaged/untracked files in local Git repositories\
Tested for Linux, FreeBSD and Windows

To try out `gogitstatus.Status()`, run the showstatus program:
```console
cd showstatus
go build
./showstatus # In any git repository
```

To try out `gogitstatus.ParseIndex()`, run the showindex program:
```console
cd showindex
go build
./showindex index
```

Run `go test` to run all the tests

## Git Index file format resources
https://git-scm.com/docs/index-format (missing some visual separation...)\
https://github.com/git/git/blob/master/read-cache.c

## TODO
- Support exclude file priority
- Support SHA-256
- Support other Git Index versions besides 2
