# go-run-python
Python embeded in Go module

## Sealing a directory into a built binary

This module can append a tar.gz payload to an already-built executable, producing a new sibling binary with a `-sealed` suffix.

- Build your app normally.
- Run a small Go program (or add a build step) that calls `gorunpython.SealDirectoryIntoBinary(binaryPath, dirPath)`.
- In your app startup, call `gorunpython.UnsealDirectoryNextToExecutableIfPresent()` to extract the sealed directory next to the running executable.

The sealed directory extracts into the same directory as the executing binary (the directory returned by `os.Executable()`), preserving the sealed directory’s root folder name.

## License Notice

Versions released after **v0.x-last-mit** are licensed under a
MIT-based license with restricted commercial use.

For commercial licensing inquiries, contact:
📧 zac@zacthe.dev
