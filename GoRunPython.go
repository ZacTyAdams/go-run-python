package gorunpython

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	var err error
	// unpack python
	var dname string
	if osName == "linux" {
		absRoot, err := filepath.Abs("/tmp/gorunpython")
		if err != nil {
			panic(err)
		}
		dname = absRoot
		if keepTemp == "" {
			_ = os.RemoveAll(dname)
		}
		if err := os.MkdirAll(dname, 0755); err != nil {
			panic(err)
		}
	} else {
		tmpDir, err := os.MkdirTemp("./", "python-tmp")
		if err != nil {
			panic(err)
		}
		dname, err = filepath.Abs(tmpDir)
		if err != nil {
			panic(err)
		}
	}
	fmt.Println("Temp dir absolute path: ", dname)

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

	// Ensure the embedded libpython is discoverable at runtime (Linux/Wolfi containers, Android)
	if osName == "linux" || osName == "android" {
		ensureEmbeddedPythonLibPath(python_bin_path)
	}
	err = makeAllFilesExecutable(python_bin_path, PythonVersion)
	if err != nil {
		panic(err)
	}

	pythonExecPath, err := resolvePythonExecutable(python_bin_path, PythonVersion)
	if err != nil {
		return nil, err
	}
	if err := ensurePipInstalled(pythonExecPath); err != nil {
		return nil, err
	}
	python_instance := &pythonInstance{
		ExtractionPath:  dname,
		Pip:             pythonExecPath + " -m pip",
		Python:          pythonExecPath,
		ExecutablesPath: python_bin_path,
		Executables:     make(map[string]pythonExecutable),
		PythonVersion:   PythonVersion,
	}
	return python_instance, nil
}

// PythonExec runs a python command using the embedded python instance
func (p *pythonInstance) PythonExec(command string) error {
	err := runPythonCommand(p.Python, []string{command}, false)
	if err != nil {
		fmt.Println("Failed to execute python command: ")
		fmt.Println(err)
	}
	return err
}

// PythonExecStream runs a python command using the embedded python instance and streams output
func (p *pythonInstance) PythonExecStream(command string) error {
	err := runPythonCommand(p.Python, []string{command}, true)
	if err != nil {
		fmt.Println("Failed to execute python command: ")
	}
	return err
}

// PipInstall installs a python package using pip in the embedded python instance
func (p *pythonInstance) PipInstall(packageName string) error {
	original_directory, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	defer os.Chdir(original_directory)
	packageArg, err := resolvePipPackageArg(packageName, original_directory)
	if err != nil {
		fmt.Println("Failed to resolve pip install package path: ")
		fmt.Println(err)
		return err
	}
	if err := os.Chdir(p.ExecutablesPath); err != nil {
		return err
	}
	err = runPythonCommand(p.Python, []string{"-m", "pip", "install", packageArg}, true)
	if err != nil {
		fmt.Println("Failed to execute pip install command: ")
		fmt.Println(err)
		currentDirectory, _ := os.Getwd()
		fmt.Println("Current directory: ", currentDirectory)
		fmt.Println("Executables path: ", p.ExecutablesPath)
		fmt.Println("Python executable: ", p.Python)
		return err
	}
	fmt.Println("Rescanning executables after pip install...")
	return p.ListExecutables()
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
	output, err := runCommand(command, args, false)
	if err != nil && shouldRetryWithLoader(command, err) {
		if loader, ok := findBundledLoader(command); ok {
			output, err = runCommand(loader, append([]string{command}, args...), false)
		}
	}
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
	err := runCommandStream(command, args)
	if err != nil && shouldRetryWithLoader(command, err) {
		if loader, ok := findBundledLoader(command); ok {
			err = runCommandStream(loader, append([]string{command}, args...))
		}
	}
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

// ensureEmbeddedPythonLibPath sets LD_LIBRARY_PATH to include the embedded python lib directory.
func ensureEmbeddedPythonLibPath(pythonBinPath string) {
	libPath := filepath.Clean(filepath.Join(pythonBinPath, "..", "lib"))
	current := os.Getenv("LD_LIBRARY_PATH")
	if current == "" {
		_ = os.Setenv("LD_LIBRARY_PATH", libPath)
		return
	}
	// Avoid duplicating the path if already present
	if !containsPath(current, libPath) {
		_ = os.Setenv("LD_LIBRARY_PATH", libPath+":"+current)
	}
}

func containsPath(list string, path string) bool {
	for _, part := range filepath.SplitList(list) {
		if part == path {
			return true
		}
	}
	return false
}

func resolvePythonExecutable(binPath string, pythonVersion string) (string, error) {
	candidates := []string{
		filepath.Join(binPath, "python"+pythonVersion),
		filepath.Join(binPath, "python3"),
		filepath.Join(binPath, "python"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("python executable not found in %s", binPath)
}

func findBundledLoader(command string) (string, bool) {
	if runtime.GOOS != "linux" {
		return "", false
	}
	binDir := filepath.Dir(command)
	libDir := filepath.Clean(filepath.Join(binDir, "..", "lib"))
	candidates := []string{
		filepath.Join(libDir, "ld-linux-aarch64.so.1"),
		filepath.Join(libDir, "ld-linux-x86-64.so.2"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func shouldRetryWithLoader(command string, err error) bool {
	if err == nil {
		return false
	}
	if _, statErr := os.Stat(command); statErr != nil {
		return false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return true
	}
	return errors.Is(err, os.ErrNotExist)
}

func runCommand(command string, args []string, stream bool) ([]byte, error) {
	cmd := exec.Command(command, args...)
	workingDir, err := os.Getwd()
	if err == nil {
		cmd.Dir = workingDir
	}
	if stream {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return nil, cmd.Run()
	}
	return cmd.CombinedOutput()
}

func runCommandStream(command string, args []string) error {
	_, err := runCommand(command, args, true)
	return err
}

func ensurePipInstalled(pythonExecPath string) error {
	if err := runPythonCommand(pythonExecPath, []string{"-m", "pip", "--version"}, false); err == nil {
		return nil
	}
	if output, err := runPythonCommandWithOutput(pythonExecPath, []string{"-m", "ensurepip", "--upgrade"}); err != nil {
		return fmt.Errorf("failed to bootstrap pip: %w\n%s", err, output)
	}
	if output, err := runPythonCommandWithOutput(pythonExecPath, []string{"-m", "pip", "--version"}); err != nil {
		return fmt.Errorf("pip still unavailable after ensurepip: %w\n%s", err, output)
	}
	return nil
}

func runPythonCommand(pythonExecPath string, args []string, stream bool) error {
	command := pythonExecPath
	commandArgs := args
	if useLoaderFor(pythonExecPath) {
		if loader, ok := findBundledLoader(pythonExecPath); ok {
			command = loader
			commandArgs = append([]string{pythonExecPath}, args...)
		}
	}
	if stream {
		return runCommandStream(command, commandArgs)
	}
	output, err := runCommand(command, commandArgs, false)
	if noisy != "" {
		fmt.Println(string(output))
	}
	return err
}

func runPythonCommandWithOutput(pythonExecPath string, args []string) (string, error) {
	command := pythonExecPath
	commandArgs := args
	if useLoaderFor(pythonExecPath) {
		if loader, ok := findBundledLoader(pythonExecPath); ok {
			command = loader
			commandArgs = append([]string{pythonExecPath}, args...)
		}
	}
	output, err := runCommand(command, commandArgs, false)
	return string(output), err
}

func useLoaderFor(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	prefix, err := readFilePrefix(path, 4)
	if err != nil {
		return false
	}
	if len(prefix) >= 2 && prefix[0] == '#' && prefix[1] == '!' {
		return false
	}
	if len(prefix) == 4 && prefix[0] == 0x7f && prefix[1] == 'E' && prefix[2] == 'L' && prefix[3] == 'F' {
		return true
	}
	return false
}

func readFilePrefix(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, n)
	read, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return buf[:read], nil
}

func resolvePipPackageArg(packageName string, originalDirectory string) (string, error) {
	if !looksLikeLocalPath(packageName) {
		return packageName, nil
	}
	if filepath.IsAbs(packageName) {
		if _, err := os.Stat(packageName); err == nil {
			return packageName, nil
		}
		return "", fmt.Errorf("package path not found: %s", packageName)
	}
	absoluteCandidate := filepath.Join(originalDirectory, packageName)
	if _, err := os.Stat(absoluteCandidate); err == nil {
		return absoluteCandidate, nil
	}
	if _, err := os.Stat(packageName); err == nil {
		return packageName, nil
	}
	return "", fmt.Errorf("package path not found: %s (cwd: %s)", packageName, originalDirectory)
}

func looksLikeLocalPath(value string) bool {
	if value == "" {
		return false
	}
	if filepath.IsAbs(value) {
		return true
	}
	if strings.Contains(value, string(os.PathSeparator)) {
		return true
	}
	if strings.HasPrefix(value, ".") {
		return true
	}
	lower := strings.ToLower(value)
	return strings.HasSuffix(lower, ".whl") || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".zip")
}
