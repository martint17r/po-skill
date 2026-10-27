package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
)

// Test hooks: how often the process ran git or touched the filesystem. Signature checks must keep both at zero.
var gitCalls, fsReads atomic.Int64

// Source is one place docs can be read from: a branch (committed content) or a directory/worktree (files on disk).
type Source struct {
	Key   string // "b:<branch>", "w:<dir>" or "d:<dir>"; the value used in the dropdown and URL
	Label string
	Ref   string // branches: full ref name
	Dir   string // worktrees: directory
}

func git(repo string, args ...string) ([]byte, error) {
	gitCalls.Add(1)
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...) // #nosec G702 G204 -- source keys and doc paths are HMAC-verified before any git call (ADR 0011)
	cmd.Env = append(os.Environ(), "LC_ALL=C")                           // fixed locale: callers match git's English messages
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// sameDir reports whether a and b are the same directory, resolving symlinks (macOS /var vs /private/var).
func sameDir(a, b string) bool {
	if ra, err := filepath.EvalSymlinks(a); err == nil {
		a = ra
	}
	if rb, err := filepath.EvalSymlinks(b); err == nil {
		b = rb
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// Sources lists the repo directory itself (first, read from disk), then its worktrees and local branches.
// Outside a git repo (or without git) only the directory is returned.
func Sources(repo string) ([]Source, error) {
	abs, err := filepath.Abs(repo)
	if err != nil {
		return nil, err
	}
	self := Source{Key: "d:" + abs, Label: "directory: " + abs, Dir: abs}
	wt, err := git(abs, "worktree", "list", "--porcelain")
	if errors.Is(err, exec.ErrNotFound) || (err != nil && strings.Contains(err.Error(), "not a git repository")) {
		return []Source{self}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Source
	var dir string
	for _, l := range strings.Split(string(wt), "\n") {
		switch {
		case strings.HasPrefix(l, "worktree "):
			dir = strings.TrimPrefix(l, "worktree ")
			out = append(out, Source{Key: "w:" + dir, Label: "worktree: " + dir, Dir: dir})
		case strings.HasPrefix(l, "branch ") && len(out) > 0:
			out[len(out)-1].Label += " (" + strings.TrimPrefix(l, "branch refs/heads/") + ")"
		case l == "bare" && len(out) > 0: // a bare repo has no files to show
			out = out[:len(out)-1]
		}
	}
	// The directory's own worktree entry is the directory source: keep it (with its branch label), listed first.
	if i := slices.IndexFunc(out, func(x Source) bool { return sameDir(x.Dir, abs) }); i >= 0 {
		x := out[i]
		out = slices.Insert(slices.Delete(out, i, i+1), 0, x)
	} else {
		out = append([]Source{self}, out...)
	}
	br, err := git(abs, "for-each-ref", "--format=%(refname)", "refs/heads")
	if err != nil {
		return nil, err
	}
	for _, ref := range strings.Fields(string(br)) {
		name := strings.TrimPrefix(ref, "refs/heads/")
		out = append(out, Source{Key: "b:" + name, Label: "branch: " + name, Ref: ref})
	}
	return out, nil
}

// isDoc reports whether p is a reviewable doc: any .md under docs/, or AGENTS.md / README.md at the root.
func isDoc(p string) bool {
	if path.Ext(p) != ".md" || !fs.ValidPath(p) {
		return false
	}
	return strings.HasPrefix(p, "docs/") || p == "AGENTS.md" || p == "README.md"
}

// Docs returns the sorted doc paths (slash separated) available in s.
func (s Source) Docs(repo string) ([]string, error) {
	var docs []string
	if s.Ref != "" {
		out, err := git(repo, "ls-tree", "-r", "--name-only", "-z", s.Ref, "--", "docs", "AGENTS.md", "README.md")
		if err != nil {
			return nil, err
		}
		for _, p := range strings.Split(string(out), "\x00") {
			if isDoc(p) {
				docs = append(docs, p)
			}
		}
	} else {
		fsReads.Add(1)
		root := os.DirFS(s.Dir)
		err := fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			if d.Type().IsRegular() && isDoc(p) {
				docs = append(docs, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(docs, func(i, j int) bool {
		// root files first, then docs/ alphabetically
		ri, rj := !strings.Contains(docs[i], "/"), !strings.Contains(docs[j], "/")
		if ri != rj {
			return ri
		}
		return docs[i] < docs[j]
	})
	return docs, nil
}

// Read returns the content of a doc. The caller must have taken p from Docs.
func (s Source) Read(repo, p string) ([]byte, error) {
	if s.Ref != "" {
		return git(repo, "show", s.Ref+":"+p)
	}
	fsReads.Add(1)
	return os.ReadFile(filepath.Join(s.Dir, filepath.FromSlash(p))) // #nosec G703 -- p is an HMAC-verified doc path from the source's own list (ADR 0011)
}
