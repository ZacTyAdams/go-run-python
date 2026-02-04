//go:build linux && arm64

package gorunpython

import _ "embed"

//go:embed universal-bucket/linux-arm64.tar.gz
var embeddedPython []byte
