- Create a test that fails when myDir() is broken on Windows (returning "." or not dealing with forward-slashes on Windows)
- fix negation bug in goignore (See: test #28)
- theres a separate bug in projects/synth concerning hashMatches(), comment out its debug code to see.
- use mywalkdir / myreaddir in fen aswell to remove the unnecessary sorting overhead
- in goignore: make fast path when pattern has no wildcards
- Add cancellation select{} block in untrackedPathsNotIgnored
- fen: add hover + click to copy on Windows



- (BAD) doing twice the work by checking if parent folders are ignored
- (BETTER) only do that work on folders AND the first entry per thread
- (BEST) fix it in goignore which would be the right thing to do.
         it won't be that slow because we use skipDir()

// keep track of ignored folders in a list/map.

if negation pattern:
	if any parent folder ignored in known list/map lookup
		skip line



My naive fix of the re-include rule has a bug with this:

.gitignore:
```
ignored_folder/
!ignored_folder/
```

files:
```
.git/
.gitignore
ignored_folder/sub/file.txt
```
