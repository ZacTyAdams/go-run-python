//go:build darwin && arm64

package gorunpython

import _ "embed"

//go:embed universal-bucket/darwin-arm64.tar.gz
var embeddedPython []byte
var PythonVersion = "3.14"
