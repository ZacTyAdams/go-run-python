package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

//go:embed darwin-arch64.tar.gz
var python_darwin []byte

type pythonInstance struct {
	extractionPath string
	pip            string
	python         string
}

func createPythonInstance() (*pythonInstance, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	fmt.Println("Go current runnon on operating system: ", osName)
	fmt.Println("Go current architecture: ", arch)
	var python_package *[]byte
	if osName == "darwin" && arch == "arm64" {
		python_package = &python_darwin
		fmt.Println("darwin arm64 python package selected")
	} else {
		panic(errors.New("unsupported operating system or architecture"))
	}
	// unpack python
	dname, err := os.MkdirTemp("./", "python-tmp")
	if err != nil {
		panic(err)
	}
	fmt.Println("Temp dir name: ", dname)

	var python_bin_path string
	err = extractTarGz(*python_package, dname)
	if err != nil {
		panic(err)
	}
	python_bin_path, err = filepath.Abs(dname + "/python/bin")
	if err != nil {
		panic(err)
	}

	err = makeAllFilesExecutable(python_bin_path)
	if err != nil {
		panic(err)
	}

	python_instance := &pythonInstance{
		extractionPath: dname,
		pip:            python_bin_path + "/pip3.10",
		python:         python_bin_path + "/python3.10",
	}
	return python_instance, nil
}

func (p *pythonInstance) PythonExec(command string) error {
	cmd := exec.Command(p.python, command)
	workingDir, err := os.Getwd()
	if err != nil {
		fmt.Println("Issue getting the current working directory")
		panic(err)
	}
	cmd.Dir = workingDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Failed to execute python command: ")
	}
	fmt.Println(string(output))
	return err
}
func (p *pythonInstance) PipInstall(packageName string) error {
	cmd := exec.Command(p.pip, "install", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Failed to execute pip install command: ")
	}
	fmt.Println(string(output))
	return err
}

// extractTarGz unpacks the embedded tar.gz data to the specified destination
func extractTarGz(data []byte, dest string) error {
	// Create a gzip reader
	gzr, err := gzip.NewReader(io.NopCloser(bytes.NewReader(data)))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	// Create a tar reader
	tr := tar.NewReader(gzr)

	// Iterate through the files in the archive
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Determine the target path
		target := filepath.Join(dest, header.Name)

		// Check the file type
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create file directory: %w", err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("failed to write file content: %w", err)
			}
			f.Close()
		}
	}
	return nil
}

func makeAllFilesExecutable(directoryPath string) error {
	// Specify the root directory to start walking from (e.g., "." for the current directory)

	// Walk through the directory tree
	err := filepath.WalkDir(directoryPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Return the error to stop walking if a serious error occurs
			return err
		}

		// Skip directories, we only want to make files executable
		if d.IsDir() {
			return nil
		}

		// Get file info to check current mode
		info, err := d.Info()
		if err != nil {
			return err
		}

		// Define the new permission mode (e.g., 0755: rwxr-xr-x)
		// This adds execute permission for all users (+x)
		newMode := info.Mode() | 0111 // 0111 is execute permission for user, group, and other

		// Change the file permissions
		err = os.Chmod(path, newMode)
		if err != nil {
			fmt.Println("Error changing mode for %s: %v\n", path, err)
			return nil // Continue walking even if one file fails
		}

		// fmt.Println("Made file executable: %s\n", path)
		return nil
	})

	if err != nil {
		fmt.Println("Error walking the directory: %v\n", err)
	}
	return nil
}

func main() {
	pythonInstance, err := createPythonInstance()
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(pythonInstance.extractionPath)
	err = pythonInstance.PipInstall("requests")
	if err != nil {
		panic(err)
	}

	err = pythonInstance.PythonExec("hello_world.py")
}
