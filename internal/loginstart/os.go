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
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

// Remove implements Files.
func (OSFiles) Remove(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
