//go:build linux && amd64

package gorunpython

import _ "embed"

//go:embed universal-bucket/linux-x86_64.tar.gz
var embeddedPython []byte
var PythonVersion = "3.10"
