- We already fixed skipping "/" in goignore, but "/////////" should also be skipped, make sure that happens.

- theres a separate bug in projects/synth concerning hashMatches(), comment out its debug code to see.
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
