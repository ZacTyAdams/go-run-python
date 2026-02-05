package main

import (
	"fmt"
	"maps"
	"os"
	"strings"

	gorunpython "github.com/ZacTyAdams/go-run-python/v2"
)

func main() {
	pythonInstance, err := gorunpython.CreatePythonInstance()
	if err != nil {
		panic(err)
	}
	// by removing or commenting out this line you can inspect the extracted python files in the temp directory
	defer os.RemoveAll(pythonInstance.ExtractionPath)

	err = pythonInstance.PipInstall("requests")
	if err != nil {
		panic(err)
	}

	err = pythonInstance.ListExecutables()
	if err != nil {
		panic(err)
	}

	var pythonExecutablePath string
	for entry := range maps.Keys(pythonInstance.Executables) {
		if strings.HasPrefix(entry, "python") {
			pythonExecutablePath = pythonInstance.Executables[entry].ExecutablePath
			break
		}
	}

	fmt.Println("Python executable path: ", pythonExecutablePath)
	// pythonExecutable.Exec([]string{"--version"})

	err = pythonInstance.PythonExecStream("hello_world.py")
	if err != nil {
		panic(err)
	}
}
