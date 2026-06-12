package atomicfile

import (
	"os"
	"path/filepath"
	"runtime"
)

// WriteFile writes data to path through a same-directory temporary file and
// atomic rename. The temporary file is fsynced before the rename and the
// parent directory after it, so a crash cannot leave the rename durable while
// the data is not.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".wsectl-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	return syncDir(dir)
}

// syncDir fsyncs a directory so a rename within it survives a crash. Windows
// cannot fsync directory handles; the rename itself is the best available
// guarantee there, so sync errors are ignored on that platform.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		if runtime.GOOS == "windows" {
			return nil
		}
		return err
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil && runtime.GOOS != "windows" {
		return err
	}
	return nil
}
