package loginstart

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

// ExecRunner runs real commands.
type ExecRunner struct{}

// Run implements Runner.
func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	// #nosec G204 -- the programs are fixed (systemctl, launchctl, schtasks.exe) and the arguments are built by this package.
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// OSFiles is the real file system.
type OSFiles struct{}

// Read implements Files.
func (OSFiles) Read(path string) ([]byte, error) { return os.ReadFile(path) }

// Write implements Files. Parent directories are created private to the user.
func (OSFiles) Write(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// Write beside the target and rename, so a crash leaves the old file or the
	// new one, and a symbolic link at path is replaced instead of followed.
	tmp, err := os.CreateTemp(dir, ".truss-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), perm); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Remove implements Files.
func (OSFiles) Remove(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
