//go:build android && arm64

package gorunpython

import _ "embed"

//go:embed universal-bucket/linux-arm64.tar.gz
var embeddedPython []byte
var PythonVersion = "3.15"
