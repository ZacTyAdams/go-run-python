//go:build darwin && amd64

package gorunpython

import _ "embed"

//go:embed universal-bucket/darwin-x86_64.tar.gz
var embeddedPython []byte
