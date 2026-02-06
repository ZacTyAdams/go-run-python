//go:build linux && arm64 && !android
// +build linux,arm64,!android

package gorunpython

import _ "embed"

//go:embed universal-bucket/linux-arm64.tar.gz
var embeddedPython []byte
var PythonVersion = "3.15"
