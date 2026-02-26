package gorunpython

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const sealTrailerMagic = "GORUNPYSEALv1\n"

// SealDirectoryIntoBinary packages dirPath as a tar.gz payload and appends it to binaryPath,
// producing a new sibling executable with a "-sealed" suffix.
//
// If binaryPath already contains a seal payload, it will be replaced.
func SealDirectoryIntoBinary(binaryPath, dirPath string) (string, error) {
	binaryInfo, err := os.Stat(binaryPath)
	if err != nil {
		return "", fmt.Errorf("stat binary: %w", err)
	}
	if binaryInfo.IsDir() {
		return "", fmt.Errorf("binary path is a directory: %s", binaryPath)
	}

	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		return "", fmt.Errorf("stat directory: %w", err)
	}
	if !dirInfo.IsDir() {
		return "", fmt.Errorf("dirPath is not a directory: %s", dirPath)
	}

	payload, err := tarGzDirectory(dirPath)
	if err != nil {
		return "", err
	}

	sealedPath := sealedSiblingPath(binaryPath)
	if err := writeSealedBinary(sealedPath, binaryPath, payload, binaryInfo.Mode()); err != nil {
		return "", err
	}
	return sealedPath, nil
}

// SealDirectoryIntoRunningExecutable seals dirPath into the currently running executable
// and writes a new "-sealed" executable next to it.
func SealDirectoryIntoRunningExecutable(dirPath string) (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return "", fmt.Errorf("resolve executable symlink: %w", err)
	}
	return SealDirectoryIntoBinary(exePath, dirPath)
}

// UnsealDirectoryNextToExecutableIfPresent checks whether the running executable contains a sealed
// payload. If so, it extracts it into the executable's directory.
//
// Returns (true, nil) if a payload was present and extracted.
func UnsealDirectoryNextToExecutableIfPresent() (bool, error) {
	exePath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("resolve executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return false, fmt.Errorf("resolve executable symlink: %w", err)
	}

	f, err := os.Open(exePath)
	if err != nil {
		return false, fmt.Errorf("open executable: %w", err)
	}
	defer f.Close()

	sealInfo, err := readSealInfo(f)
	if err != nil {
		return false, err
	}
	if sealInfo == nil {
		return false, nil
	}

	payload := make([]byte, sealInfo.payloadSize)
	if _, err := f.ReadAt(payload, sealInfo.payloadOffset); err != nil {
		return false, fmt.Errorf("read sealed payload: %w", err)
	}

	destDir := filepath.Dir(exePath)
	if err := extractTarGzSafe(payload, destDir); err != nil {
		return false, err
	}
	return true, nil
}

type sealInfo struct {
	payloadOffset int64
	payloadSize   int64
}

func readSealInfo(f *os.File) (*sealInfo, error) {
	st, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat executable: %w", err)
	}
	size := st.Size()
	trailerLen := int64(len(sealTrailerMagic) + 8)
	if size < trailerLen {
		return nil, nil
	}

	trailer := make([]byte, trailerLen)
	if _, err := f.ReadAt(trailer, size-trailerLen); err != nil {
		return nil, fmt.Errorf("read seal trailer: %w", err)
	}
	magic := string(trailer[:len(sealTrailerMagic)])
	if magic != sealTrailerMagic {
		return nil, nil
	}
	payloadSizeU64 := binary.LittleEndian.Uint64(trailer[len(sealTrailerMagic):])
	if payloadSizeU64 == 0 {
		return nil, fmt.Errorf("invalid sealed payload size (0)")
	}
	if payloadSizeU64 > uint64(size-trailerLen) {
		return nil, fmt.Errorf("invalid sealed payload size (%d) for file size (%d)", payloadSizeU64, size)
	}
	payloadSize := int64(payloadSizeU64)
	payloadOffset := size - trailerLen - payloadSize
	if payloadOffset < 0 {
		return nil, fmt.Errorf("invalid sealed payload offset")
	}
	return &sealInfo{payloadOffset: payloadOffset, payloadSize: payloadSize}, nil
}

func sealedSiblingPath(binaryPath string) string {
	dir := filepath.Dir(binaryPath)
	base := filepath.Base(binaryPath)
	if strings.EqualFold(filepath.Ext(base), ".exe") {
		name := strings.TrimSuffix(base, filepath.Ext(base))
		return filepath.Join(dir, name+"-sealed.exe")
	}
	return filepath.Join(dir, base+"-sealed")
}

func writeSealedBinary(outPath, inPath string, payload []byte, mode os.FileMode) error {
	in, err := os.Open(inPath)
	if err != nil {
		return fmt.Errorf("open input binary: %w", err)
	}
	defer in.Close()

	baseSize, err := sealedBaseSize(in)
	if err != nil {
		return err
	}
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek input binary: %w", err)
	}

	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("create output binary: %w", err)
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.CopyN(out, in, baseSize); err != nil {
		return fmt.Errorf("copy base binary: %w", err)
	}
	if _, err := out.Write(payload); err != nil {
		return fmt.Errorf("write sealed payload: %w", err)
	}
	if _, err := out.WriteString(sealTrailerMagic); err != nil {
		return fmt.Errorf("write seal magic: %w", err)
	}
	var sizeBuf [8]byte
	binary.LittleEndian.PutUint64(sizeBuf[:], uint64(len(payload)))
	if _, err := out.Write(sizeBuf[:]); err != nil {
		return fmt.Errorf("write seal size: %w", err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("close output binary: %w", err)
	}
	return nil
}

func sealedBaseSize(f *os.File) (int64, error) {
	st, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat input binary: %w", err)
	}
	size := st.Size()
	info, err := readSealInfo(f)
	if err != nil {
		return 0, err
	}
	if info == nil {
		return size, nil
	}
	trailerLen := int64(len(sealTrailerMagic) + 8)
	return size - trailerLen - info.payloadSize, nil
}

func tarGzDirectory(dirPath string) ([]byte, error) {
	rootAbs, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute directory: %w", err)
	}
	rootName := filepath.Base(rootAbs)

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	closeAll := func(closeErr error) ([]byte, error) {
		_ = tw.Close()
		_ = gzw.Close()
		if closeErr != nil {
			return nil, closeErr
		}
		return buf.Bytes(), nil
	}

	walkErr := filepath.WalkDir(rootAbs, func(fullPath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not supported in sealed directories: %s", fullPath)
		}

		rel, err := filepath.Rel(rootAbs, fullPath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		name := rootName
		if rel != "." {
			name = rootName + "/" + rel
		}
		if info.IsDir() {
			if !strings.HasSuffix(name, "/") {
				name += "/"
			}
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type in sealed directory: %s", fullPath)
		}
		file, err := os.Open(fullPath)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		return nil
	})
	if walkErr != nil {
		return closeAll(walkErr)
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func extractTarGzSafe(data []byte, dest string) error {
	gzr, err := gzip.NewReader(io.NopCloser(bytes.NewReader(data)))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("resolve absolute dest: %w", err)
	}

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		clean := path.Clean(hdr.Name)
		if clean == "." {
			continue
		}
		if strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("invalid path in sealed archive: %q", hdr.Name)
		}

		target := filepath.Join(destAbs, filepath.FromSlash(clean))
		rel, err := filepath.Rel(destAbs, target)
		if err != nil {
			return fmt.Errorf("validate path in archive: %w", err)
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("invalid path traversal in sealed archive: %q", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("failed to create file directory: %w", err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return fmt.Errorf("failed to write file content: %w", err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("failed to close extracted file: %w", err)
			}
		default:
			return fmt.Errorf("unsupported entry type in sealed archive (%c) for %q", hdr.Typeflag, hdr.Name)
		}
	}
	return nil
}
