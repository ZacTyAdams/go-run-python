package main

import (
	"os"

	gorunpython "github.com/ZacTyAdams/go-run-python/m/v2"
)

func main() {
	pythonInstance, err := gorunpython.CreatePythonInstance()
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(pythonInstance.ExtractionPath)
	err = pythonInstance.PipInstall("requests")
	if err != nil {
		panic(err)
	}

	err = pythonInstance.PythonExec("hello_world.py")
	if err != nil {
		panic(err)
	}
}
