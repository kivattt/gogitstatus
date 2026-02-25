If you configure the line endings of a file to be normalized in something like `.gitattributes`, the SHA-1 hash in the `.git/index` may be different to the normal hash of the file.

<details>
<summary>Demonstration of the SHA-1 hash being different than usual (bash)</summary>

```
$ mkdir tmp
$ unzip files.zip -d tmp/
$ cd tmp/
$ git hash-object crlf_file_with_lf_in_index.txt
f98585722413e2775399bf49fd987a837d5aad5a
$ (printf "blob 48\x00" && cat crlf_file_with_lf_in_index.txt) | sha1sum
4be2235c15a51bf739255091820a47b34c3e61e0  -
```

The number 48 in `printf "blob 48\x00"` is the size of the file, 48 bytes.
</details>

This is because when `git add` is called on the file, a blob in the `.git/objects/` is created with it's line endings normalized, and the `.git/index` stores the hash of the normalized blob.

[What's a blob?](https://git-scm.com/book/en/v2/Git-Internals-Git-Objects)

Normally this problem won't show up in gogitstatus since the timestamp remains the same and no hashing is done.
However, if the entire repository folder is copied on Linux (without `cp -p # preserve flag`) the timestamp of the files are changed, forcing us to do the hash check.

It's at this point we need to implement logic for deciding if, and how to convert line endings before hashing.

Currently (25. feb 2026 03:34:10 in Norway), we don't handle this properly, and hash the file as if it wasn't normalized!

## The hacky solution

If the hash doesn't match, try to hash it with line endings normalized.\
Try both LF and CRLF. Starting with whichever is the most common, like the default option in git for windows. Are there more line endings than LF and CRLF?

## The real solution

We need to parse `.gitattributes` files per-directory like we do for `.gitignore` and use that to determine if and how to convert line endings before hashing.

There are also the other default paths for this kind of stuff, maybe even in the `.git/config` which we'd _also_ have to parse to do it correctly.

## Hypothesis
`git status` notices the hash matches even though the timestamp didn't.\
It then updates the timestamp and all other stat info for the file in the `.git/index` because the hash matched.

I think this is why we see the `.git/index` change whenever `git status` is ran after copying the repo folder.

I think this is also why running `git status` in a repo like this usually "fixes" the bug in gogitstatus, although I have one mystery repo where it doesn't get fixed no matter how many times I run `git status`...
