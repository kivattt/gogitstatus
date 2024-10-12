To test the functionality of `gogitstatus.StatusRaw()`, run the showstatusraw program:
```console
cd showstatusraw
go build
./showstatusraw files index
```

To test the functionality of `gogitstatus.ParseIndex()`, run the showindex program:
```console
cd showindex
go build
./showindex index
```
An example Git index file has been provided in `showindex/index`

## TODO
- Respect .gitignore
- Support SHA-256
- Support other Git Index versions besides 2
