Remember the `-y` flag to allow zipping symlinks
```
zip -y -r files.zip * .git
```

The `tests-statusraw/2_missing_files/files.zip` file is an empty zip file generated with this:\
https://stackoverflow.com/a/64466237

Contents of that link, copied below:
```
I have done it the dirty way:

Created an file called emptyfile with one byte in it (whitespace).

Then I added that file into the zip file:

zip -q {{project}}.zip emptyfile

To finally remove it:

zip -dq {{project}}.zip emptyfile

(if -q is omitted, you will get warning: zip warning: zip file empty)

This way I've got an empty zip file one can -update.

All that can be converted to a oneliner (idea from GaspardP):

echo | zip -q > {{project}}.zip && zip -dq {{project}}.zip -

Is there a more elegant way to do this?
```
