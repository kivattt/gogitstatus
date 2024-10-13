This is a library for finding unstaged/uncommited files in local Git repositories using zero dependencies, only the Go standard library

To try out `gogitstatus.StatusRaw()`, run the showstatusraw program:
```console
cd showstatusraw
go build
./showstatusraw files index
```

To try out `gogitstatus.ParseIndex()`, run the showindex program:
```console
cd showindex
go build
./showindex index
```

Run `go test` to run all the tests

## Git Index file format resources
https://git-scm.com/docs/index-format (a little confusing)\
https://github.com/git/git/blob/master/read-cache.c

## TODO
- Respect .gitignore
- Support SHA-256
- Support other Git Index versions besides 2
