This test makes sure we remember to add a "/" suffix to all strings in `gitLinkPaths`.

Because otherwise, it can match a file with the same prefix when it should only match the folder and files within it.

Correct:   `strings.HasPrefix("tutils2_new_file", "tutils2/") == false`
Incorrect: `strings.HasPrefix("tutils2_new_file", "tutils2") == true`
