package main

import (
	"fmt"
	"maps"
	"os"

	gorunpython "github.com/ZacTyAdams/go-run-python/v2"
)

func main() {
	// This will print extra log and output formation from the pyton scripts
	// you can also use ExecStream to get the full live output from the python script without the extra go noise
	// if you want silence just remove this line and environment variable
	os.Setenv("GORUNPYTHON_NOISY", "true")

	pythonInstance, err := gorunpython.CreatePythonInstance()
	if err != nil {
		panic(err)
	}
	fmt.Println("Python version: ", pythonInstance.PythonVersion)
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

	var targetExecutable string
	for entry := range maps.Keys(pythonInstance.Executables) {
		if entry == "python"+pythonInstance.PythonVersion {
			targetExecutable = entry
			break
		}
	}

	pythonExecutable := pythonInstance.Executables[targetExecutable]
	pythonExecutable.ExecStream([]string{"--version"})

	err = pythonInstance.PythonExecStream("hello_world.py")
	if err != nil {
		panic(err)
	}
}
