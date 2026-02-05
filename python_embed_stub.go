//go:build !(darwin && arm64) && !(darwin && amd64) && !(linux && amd64) && !(linux && arm64) && !(linux && arm) && !(windows && amd64)

package gorunpython

// Fallback stub: when no platform-specific embed file is present this
// variable will be empty and the runtime will error with a helpful message.
var embeddedPython []byte
var PythonVersion = "3.10"
