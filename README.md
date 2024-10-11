To test the functionality of `gogitstatus.Status()`, run the showstatus program:
```console
cd showstatus
go build
./showstatus git-repository
```
An example Git repository has been provided in `showindex/git-repository`

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
