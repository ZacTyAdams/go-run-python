package main

import (
	"fmt"

	gorunpython "github.com/ZacTyAdams/go-run-python/v2"
)

func main() {
	pythonInstance, err := gorunpython.CreatePythonInstance()
	if err != nil {
		panic(err)
	}
	// defer os.RemoveAll(pythonInstance.ExtractionPath)
	err = pythonInstance.PipInstall("requests")
	if err != nil {
		panic(err)
	}

	err = pythonInstance.ListExecutables()
	if err != nil {
		panic(err)
	}

	pythonExecutable := pythonInstance.Executables["python3.10"]

	fmt.Println("Python executable path: ", pythonExecutable.ExecutablePath)
	pythonExecutable.Exec([]string{"--version"})

	err = pythonInstance.PythonExecStream("hello_world.py")
	if err != nil {
		panic(err)
	}
}
