package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	fmt.Println("Hello, World!")

	exePath, err := os.Executable()
	if err != nil {
		fmt.Println("Error getting executable path:", err)
		os.Exit(1)
	}
	exeDir := filepath.Dir(exePath)

	// Layout assumption:
	// <root>/python/bin/python-launcher   (this binary)
	// <root>/python/bin/python3.14
	// <root>/python/lib/ld-linux-*.so.*
	// <root>/python/lib/libc.so.6, etc
	pythonBinDir := exeDir
	pythonRoot := filepath.Clean(filepath.Join(pythonBinDir, ".."))
	pythonLibDir := filepath.Join(pythonRoot, "lib")
	pythonExe := filepath.Join(pythonBinDir, "python3.14")

	fmt.Printf("Python root: %s\n", pythonRoot)
	fmt.Printf("Python bin:  %s\n", pythonBinDir)
	fmt.Printf("Python lib:  %s\n", pythonLibDir)
	fmt.Printf("Python exe:  %s\n", pythonExe)

	// Pick the right dynamic loader (interpreter) bundled with your python
	var loaderName string
	switch runtime.GOARCH {
	case "amd64":
		loaderName = "ld-linux-x86-64.so.2"
	case "arm64":
		loaderName = "ld-linux-aarch64.so.1"
	default:
		fmt.Printf("Packaged interpreter not available for architecture: %s\n", runtime.GOARCH)
		os.Exit(1)
	}
	loaderPath := filepath.Join(pythonLibDir, loaderName)
	fmt.Printf("Loader:      %s\n", loaderPath)

	// Pass through args to python
	pyArgs := os.Args[1:]

	// IMPORTANT: Run the loader directly and control the library search path tightly
	cmdArgs := append([]string{
		"--library-path", pythonLibDir,
		"--inhibit-cache",
		pythonExe,
	}, pyArgs...)

	cmd := exec.Command(loaderPath, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Avoid mixing host/container glibc/musl bits:
	env := os.Environ()
	env = scrubEnv(env, []string{"LD_LIBRARY_PATH", "LD_PRELOAD", "LD_AUDIT"})
	env = append(env,
		"LD_LIBRARY_PATH="+pythonLibDir, // keep it tight
		// These help Python find its stdlib predictably when invoked via loader
		"PYTHONHOME="+pythonRoot,
	)
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		fmt.Println("Error running command:", err)
		os.Exit(1)
	}

	fmt.Println("Done!")
}

func scrubEnv(env []string, keys []string) []string {
	kill := map[string]bool{}
	for _, k := range keys {
		kill[k+"="] = true
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		skip := false
		for prefix := range kill {
			if strings.HasPrefix(kv, prefix) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, kv)
		}
	}
	return out
}
