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
	syncRoot, err := mkdirAllTracked(dir)
	if err != nil {
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
	return syncDirChain(dir, syncRoot)
}

// mkdirAllTracked creates dir like os.MkdirAll and returns the parent of the
// topmost directory it had to create, or "" if dir already existed. That
// parent holds the directory entry for the new chain and must be fsynced for
// the chain to survive a crash.
func mkdirAllTracked(dir string) (string, error) {
	topMissing := ""
	for d := dir; ; {
		if _, err := os.Stat(d); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return "", err
		}
		topMissing = d
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if topMissing == "" {
		return "", nil
	}
	return filepath.Dir(topMissing), nil
}

// syncDirChain fsyncs dir and, when WriteFile created missing parents, each
// ancestor up to and including syncRoot, so newly created directory entries
// are durable along with the renamed file.
func syncDirChain(dir, syncRoot string) error {
	if err := syncDir(dir); err != nil {
		return err
	}
	for d := dir; syncRoot != "" && d != syncRoot; {
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		if err := syncDir(parent); err != nil {
			return err
		}
		d = parent
	}
	return nil
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
