//go:build windows && amd64

package gorunpython

import _ "embed"

//go:embed universal-bucket/windows-x86_64.tar.gz
var embeddedPython []byte
var PythonVersion = "3.10"
