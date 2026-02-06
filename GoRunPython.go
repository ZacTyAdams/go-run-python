package gorunpython

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

var noisy = os.Getenv("GORUNPYTHON_NOISY")
var keepTemp = os.Getenv("GORUNPYTHON_KEEP_TEMP")

type pythonInstance struct {
	ExtractionPath  string
	Pip             string
	Python          string
	ExecutablesPath string
	Executables     map[string]pythonExecutable
	PythonVersion   string
}

type pythonExecutable struct {
	ExecutableName string
	ExecutablePath string
}

// CreatePythonInstance unpacks the appropriate embedded python package for the current OS and architecture
func CreatePythonInstance() (*pythonInstance, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	fmt.Println("Go current runnon on operating system: ", osName)
	fmt.Println("Go current architecture: ", arch)
	fmt.Println("Selecting appropriate embedded python package...")

	// select embedded python package for this build (set via build-tag specific file)
	if len(embeddedPython) == 0 {
		return nil, fmt.Errorf("no embedded python package for %s-%s; add an embed file with matching //go:build or build for a supported target", osName, arch)
	}
	python_package := embeddedPython
	// unpack python
	dname, err := os.MkdirTemp("./", "python-tmp")
	if err != nil {
		panic(err)
	}
	fmt.Println("Temp dir name: ", dname)

	// old way to unpack
	err = extractTarGz(python_package, dname)
	if err != nil {
		panic(err)
	}

	var python_bin_path string
	if osName == "darwin" || osName == "android" {
		python_bin_path, err = filepath.Abs(filepath.Join(dname, "/prefix/bin"))
		if err != nil {
			panic(err)
		}
	} else {
		python_bin_path, err = filepath.Abs(filepath.Join(dname, "/python/bin"))
		if err != nil {
			panic(err)
		}
	}

	err = makeAllFilesExecutable(python_bin_path, PythonVersion)
	if err != nil {
		panic(err)
	}
	python_instance := &pythonInstance{
		ExtractionPath:  dname,
		Pip:             filepath.Join(python_bin_path, "/python"+PythonVersion) + " -m pip",
		Python:          filepath.Join(python_bin_path, "/python"+PythonVersion),
		ExecutablesPath: python_bin_path,
		Executables:     make(map[string]pythonExecutable),
		PythonVersion:   PythonVersion,
	}
	return python_instance, nil
}

// PythonExec runs a python command using the embedded python instance
func (p *pythonInstance) PythonExec(command string) error {
	err := executeCommand(p.Python, []string{command})
	if err != nil {
		fmt.Println("Failed to execute python command: ")
	}
	return err
}

// PythonExecStream runs a python command using the embedded python instance and streams output
func (p *pythonInstance) PythonExecStream(command string) error {
	err := executeCommandStream(p.Python, []string{command})
	if err != nil {
		fmt.Println("Failed to execute python command: ")
	}
	return err
}

// PipInstall installs a python package using pip in the embedded python instance
func (p *pythonInstance) PipInstall(packageName string) error {
	err := executeCommandStream(p.Python, []string{"-m", "pip", "install", packageName})
	if err != nil {
		fmt.Println("Failed to execute pip install command: ")
	}
	fmt.Println("Rescanning executables after pip install...")
	err = p.ListExecutables()
	return err
}

// ListExecutables lists all executables in the embedded python instance's bin directory and stores them in the pythonInstance.Executables map
func (p *pythonInstance) ListExecutables() error {
	files, err := os.ReadDir(p.ExecutablesPath)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		execPath := filepath.Join(p.ExecutablesPath, file.Name())

		p.Executables[file.Name()] = pythonExecutable{ExecutableName: file.Name(), ExecutablePath: execPath}
		if noisy != "" {
			fmt.Println("Found executable: ", file.Name())
		}
	}

	return err
}

// Exec runs a command using the specified pythonExecutable.ExecutablePath
func (e *pythonExecutable) Exec(args []string) error {
	err := executeCommand(e.ExecutablePath, args)
	if err != nil {
		fmt.Println("Failed to execute python executable command: ")
	}
	return err
}

// ExecStream runs a command using the specified pythonExecutable.ExecutablePath and streams output
func (e *pythonExecutable) ExecStream(args []string) error {
	// We assume noisy is always true for streaming
	err := executeCommandStream(e.ExecutablePath, args)
	if err != nil {
		fmt.Println("Failed to execute python executable command:")
	}
	return err
}

// executeCommand is an internal helper function to execute a command and return its output
func executeCommand(command string, args []string) error {
	cmd := exec.Command(command, args...)
	WorkingDir, err := os.Getwd()
	cmd.Dir = WorkingDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Failed to execute command: ", command)
	}
	if noisy != "" {
		fmt.Println(string(output))
	}
	return err
}

// executeCommandStream is an internal helper function to execute a command and stream its output
func executeCommandStream(command string, args []string) error {
	// We assume noisy is always true for streaming
	cmd := exec.Command(command, args...)
	WorkingDir, err := os.Getwd()
	cmd.Dir = WorkingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Println("Failed to execute command: ", command)
	}
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

// makeAllFilesExecutable makes all files in the specified directory executable
func makeAllFilesExecutable(directoryPath string, pythonVersion string) error {
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

		// Correct the file's shebang and other instances referring to the original build path at #!/Users/zacadams/Development/cpython-android/build-macos/../out/prefix/bin/python3.15
		// This is necessary because the embedded python may have hardcoded paths that don't match the temp directory structure
		// We will replace any instance of the original build path with the new temp directory path
		originalBuildPath := "/Users/zacadams/Development/cpython-android/build-macos/../out/prefix/bin/python" + pythonVersion
		newPath := filepath.Join(directoryPath, "/python"+pythonVersion)
		input, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("Error reading file for shebang correction %s: %v\n", path, err)
			return nil // Continue walking even if one file fails
		}
		output := bytes.ReplaceAll(input, []byte(originalBuildPath), []byte(newPath))
		err = os.WriteFile(path, output, newMode)
		if noisy != "" {
			fmt.Printf("Corrected pathes and permissions in file: %s\n", path)
		}
		if err != nil {
			fmt.Printf("Error writing file for shebang correction %s: %v\n", path, err)
			return nil // Continue walking even if one file fails
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking the directory: %v\n", err)
	}
	return nil
}
