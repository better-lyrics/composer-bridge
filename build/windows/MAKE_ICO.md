# Windows .ico assembly

`icotool` was not on PATH when icons were regenerated, so the PNG layers in this
directory have not been packed into a single `icon.ico`. Install `icoutils` and
run the one-liner below.

```
brew install icoutils
icotool -c -o icon.ico icon-16.png icon-24.png icon-32.png icon-48.png icon-64.png icon-256.png
```

Any equivalent tool works as well (for example ImageMagick:
`magick convert icon-16.png icon-24.png icon-32.png icon-48.png icon-64.png icon-256.png icon.ico`).

Re-running `scripts/regen-icons.sh` after installing `icotool` will produce the
`.ico` automatically and remove this note.
