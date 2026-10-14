// Package safepath resolves the files MIA writes for manual handoff. Exports
// must never land inside the read-only Punyamas workspace, and draft exports
// must stay inside the configured drafts directory (no absolute names, no
// "..", no symlink escapes).
package safepath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveExisting resolves symlinks in the deepest existing ancestor of p and re-appends the rest,
// so paths that do not exist yet are compared on their real location.
func resolveExisting(p string) string {
	p = filepath.Clean(p)
	var rest []string
	cur := p
	for {
		if r, err := filepath.EvalSymlinks(cur); err == nil {
			for i := len(rest) - 1; i >= 0; i-- {
				r = filepath.Join(r, rest[i])
			}
			return r
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p
		}
		rest = append(rest, filepath.Base(cur))
		cur = parent
	}
}

// Under reports whether path lies inside root (or is root), comparing real locations.
func Under(path, root string) bool {
	if root == "" || path == "" {
		return false
	}
	r, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	p, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	r, p = resolveExisting(r), resolveExisting(p)
	return p == r || strings.HasPrefix(p, r+string(filepath.Separator))
}

// ErrRefused marks an export target that the path rules do not allow.
var ErrRefused = errors.New("tujuan ekspor ditolak")

// ResolveDraftPath resolves a relative file name under base. Absolute names, any ".." element,
// targets that escape base through symlinks, and any target inside workspaceRoot (also when base
// itself was configured inside the workspace) are refused. ".md" is appended when missing.
func ResolveDraftPath(base, name, workspaceRoot string) (string, error) {
	if base == "" {
		return "", fmt.Errorf("%w: direktori draf tidak dikonfigurasi", ErrRefused)
	}
	if name == "" || filepath.IsAbs(name) || strings.HasPrefix(name, "~") || !filepath.IsLocal(name) {
		return "", fmt.Errorf("%w: %q harus berupa nama relatif di dalam direktori draf (tanpa path absolut atau '..')", ErrRefused, name)
	}
	for _, part := range strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return "", fmt.Errorf("%w: %q mengandung '..'", ErrRefused, name)
		}
	}
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		name += ".md"
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	target := filepath.Join(absBase, name)
	realBase, realTarget := resolveExisting(absBase), resolveExisting(target)
	if !strings.HasPrefix(realTarget, realBase+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q keluar dari direktori draf", ErrRefused, name)
	}
	if workspaceRoot != "" && (Under(realTarget, workspaceRoot) || Under(realBase, workspaceRoot)) {
		return "", fmt.Errorf("%w: %s berada di dalam ruang kerja Punyamas (baca-saja); ekspor ke luar ruang kerja lalu serahkan manual", ErrRefused, realTarget)
	}
	return target, nil
}

// WriteAtomic writes data to path through a temporary file in the same directory and a rename,
// so a failed export never leaves a partial file behind.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".mia-export-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, perm); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
