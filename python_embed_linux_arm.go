//go:build linux && arm

package gorunpython

import _ "embed"

//go:embed universal-bucket/linux-arm7.tar.gz
var embeddedPython []byte
