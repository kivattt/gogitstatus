- The bug in projects/synth is because of .gitattributes doing `text eol=crlf` which allows the file to differ from the .git/objects/ it belongs to, e.g. if hashMatches() doesn't match, we should re-check with CRLF line-endings converted (HACKY, since the correct way would be to parse .gitattributes respecting the proper hierarchy).
- Do an early return if stat filesize differs, before we do a hash check in hashMatches(). Then remove the TODO: comment for that

- We already fixed skipping "/" in goignore, but "/////////" should also be skipped, make sure that happens.
- use mywalkdir / myreaddir in fen aswell to remove the unnecessary sorting overhead
- in goignore: make fast path when pattern has no wildcards?
- Add cancellation select{} block in untrackedPathsNotIgnored
- Make test output better (horizontal 1, 2, 3... instead of taking up so many lines, or omitting successful tests)
- Make a visualization tool for cancellation latencies at different intervals (probably just write a .csv we can graph using a tool)


- (BAD) doing twice the work by checking if parent folders are ignored
- (BETTER) only do that work on folders AND the first entry per thread
- (BEST) fix it in goignore which would be the right thing to do.
         it won't be that slow because we use skipDir()

// keep track of ignored folders in a list/map.

if negation pattern:
	if any parent folder ignored in known list/map lookup
		skip line
