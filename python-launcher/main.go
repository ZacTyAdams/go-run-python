package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// the goal for this function will be to seekout and pathmap the correct python executable and it's related dependencies
// var pythonPath = "python/bin"
var executableDir string
var pythonLibPath string

func getPythonLibPath() string {
	relativePythonLibPath := "../lib"
	executableDir, err := os.Executable()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		return ""
	}
	return filepath.Join(filepath.Join(executableDir, ".."), relativePythonLibPath)
}

func getInterpreterPath() string {
	archecture := runtime.GOARCH
	var interpreterName string
	switch archecture {
	case "amd64":
		interpreterName = "ld-linux-x86-64.so.2"
	case "arm64":
		interpreterName = "ld-linux-aarch64.so.1"
	default:
		fmt.Printf("Packaged interpreter not available for architecture: %s\n", archecture)
		return ""
	}
	fmt.Println("pythonLibPath: ", pythonLibPath)
	interpreterPath := filepath.Join(pythonLibPath, interpreterName)

	return interpreterPath
}

func main() {
	fmt.Println("Hello, World!")
	// err := os.Chdir(pythonPath)
	// if err != nil {
	// 	fmt.Println("Error changing directory:", err)
	// }
	executableDir, err := os.Executable()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
	}
	pythonLibPath = getPythonLibPath()
	fmt.Printf("Python library path: %s\n", pythonLibPath)

	os.Setenv("LD_LIBRARY_PATH", pythonLibPath+":"+os.Getenv("LD_LIBRARY_PATH"))
	interpreterPath := getInterpreterPath()
	fmt.Printf("Interpreter path: %s\n", interpreterPath)
	fmt.Printf("Current working directory: %s\n", executableDir)
	pythonExecutable := filepath.Join(filepath.Join(executableDir, ".."), "python3.14")
	// This works, now add option for passing args to the python executable
	args := os.Args[1:]
	cmd := exec.Command(interpreterPath, append([]string{"--library-path", pythonLibPath, pythonExecutable}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error running command:", err)
	}
	fmt.Println("Done!")

}
