// Package agent runs inside every workspace pod. It exposes the workspace
// filesystem and an interactive PTY over a single WebSocket that the control
// plane dials and proxies to the browser IDE. It is deliberately dependency-
// light (no k8s, no db) so the workspace image stays small.
package agent

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DirEntry is one item in a directory listing.
type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// FS provides filesystem operations confined to a root directory (the
// workspace). Every path is resolved and checked so a client cannot escape
// the root via ".." or absolute paths.
type FS struct{ root string }

// NewFS returns an FS rooted at root (e.g. /workspace).
func NewFS(root string) *FS { return &FS{root: root} }

// resolve maps a client path (relative to the workspace root) to an absolute
// on-disk path, guaranteeing the result stays within root.
func (f *FS) resolve(p string) (string, error) {
	// Clean against "/" collapses any leading ".." so the join can't escape.
	clean := filepath.Clean("/" + strings.TrimSpace(p))
	full := filepath.Join(f.root, clean)
	rel, err := filepath.Rel(f.root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("path escapes workspace")
	}
	return full, nil
}

// List returns the entries of a directory (sorted: dirs first, then by name).
func (f *FS) List(p string) ([]DirEntry, error) {
	full, err := f.resolve(p)
	if err != nil {
		return nil, err
	}
	items, err := os.ReadDir(full)
	if err != nil {
		return nil, err
	}
	out := make([]DirEntry, 0, len(items))
	for _, it := range items {
		info, err := it.Info()
		var size int64
		if err == nil {
			size = info.Size()
		}
		out = append(out, DirEntry{Name: it.Name(), IsDir: it.IsDir(), Size: size})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir // directories first
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// Read returns the contents of a file.
func (f *FS) Read(p string) ([]byte, error) {
	full, err := f.resolve(p)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

// Write creates or overwrites a file, creating parent directories as needed.
func (f *FS) Write(p string, data []byte) error {
	full, err := f.resolve(p)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

// Mkdir creates a directory (and any missing parents).
func (f *FS) Mkdir(p string) error {
	full, err := f.resolve(p)
	if err != nil {
		return err
	}
	return os.MkdirAll(full, 0o755)
}

// Delete removes a file or directory tree.
func (f *FS) Delete(p string) error {
	full, err := f.resolve(p)
	if err != nil {
		return err
	}
	if full == f.root {
		return errors.New("refusing to delete workspace root")
	}
	return os.RemoveAll(full)
}

// Rename moves a file or directory within the workspace.
func (f *FS) Rename(p, to string) error {
	src, err := f.resolve(p)
	if err != nil {
		return err
	}
	dst, err := f.resolve(to)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}
