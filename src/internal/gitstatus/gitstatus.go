// Package gitstatus provides bounded, shell-free access to a Git worktree's
// status and diff, plus explicit path-scoped staging actions.
package gitstatus

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultTimeout   = 5 * time.Second
	defaultMaxOutput = int64(8 << 20)
	literalPathspecs = "--literal-pathspecs"
)

// Options controls the resource bounds used for each Git command. Zero values
// select conservative defaults.
type Options struct {
	Timeout   time.Duration
	MaxOutput int64
}

// Client runs Git operations with fixed resource bounds.
type Client struct {
	timeout   time.Duration
	maxOutput int64
	gitBinary string
}

// New creates a Git status client. Invalid non-positive option values use the
// defaults so a zero Options value is always ready to use.
func New(options Options) *Client {
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	maxOutput := options.MaxOutput
	if maxOutput <= 0 {
		maxOutput = defaultMaxOutput
	}
	return &Client{timeout: timeout, maxOutput: maxOutput, gitBinary: "git"}
}

// Root returns the absolute repository root containing path. A file, a
// directory, or a not-yet-created descendant may be supplied.
func (c *Client) Root(ctx context.Context, path string) (string, error) {
	dir, err := existingDirectory(path)
	if err != nil {
		return "", newError("find repository root", path, InvalidPath, err, "")
	}
	out, err := c.run(ctx, "find repository root", path, dir, true, "rev-parse", "--show-toplevel")
	if err != nil {
		var gitErr *Error
		if errors.As(err, &gitErr) && gitErr.Kind == CommandFailed {
			return "", newError("find repository root", path, NotRepository, gitErr, gitErr.Stderr)
		}
		return "", err
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", newError("find repository root", path, NotRepository, errors.New("git returned an empty root"), "")
	}
	abs, err := filepath.Abs(filepath.FromSlash(root))
	if err != nil {
		return "", newError("find repository root", path, InvalidPath, err, "")
	}
	return filepath.Clean(abs), nil
}

// Status reads the complete repository status, including ignored and
// untracked files. It never refreshes or writes the index.
func (c *Client) Status(ctx context.Context, path string) (Snapshot, error) {
	root, err := c.Root(ctx, path)
	if err != nil {
		return Snapshot{}, err
	}
	out, err := c.run(ctx, "read status", root, root, true,
		literalPathspecs, "status", "--porcelain=v2", "-z",
		"--untracked-files=all", "--ignored=matching")
	if err != nil {
		return Snapshot{}, err
	}
	entries, err := parsePorcelainV2(out)
	if err != nil {
		return Snapshot{}, newError("parse status", root, InvalidOutput, err, "")
	}
	return Snapshot{Root: root, Entries: entries}, nil
}

// DiffOptions selects an index diff and optionally restricts it to literal
// repository paths. With no paths, Diff returns the whole repository diff.
type DiffOptions struct {
	Staged bool
	Paths  []string
}

// Diff returns a bounded Git patch without invoking external diff or textconv
// helpers.
func (c *Client) Diff(ctx context.Context, path string, options DiffOptions) ([]byte, error) {
	root, err := c.Root(ctx, path)
	if err != nil {
		return nil, err
	}
	paths, err := normalizePaths(root, options.Paths, true)
	if err != nil {
		return nil, err
	}
	args := []string{literalPathspecs, "diff", "--no-ext-diff", "--no-textconv", "--no-color"}
	if options.Staged {
		args = append(args, "--cached")
	}
	args = append(args, "--")
	args = append(args, paths...)
	return c.run(ctx, "read diff", root, root, true, args...)
}

// Stage adds only the explicitly supplied literal paths to the index.
func (c *Client) Stage(ctx context.Context, path string, paths ...string) error {
	return c.mutate(ctx, "stage", path, paths, func(root string, normalized []string) error {
		args := append([]string{literalPathspecs, "add", "--"}, normalized...)
		_, err := c.run(ctx, "stage", root, root, false, args...)
		return err
	})
}

// Unstage removes only the explicitly supplied literal paths from the index,
// preserving their worktree contents. It also supports repositories without a
// first commit.
func (c *Client) Unstage(ctx context.Context, path string, paths ...string) error {
	return c.mutate(ctx, "unstage", path, paths, func(root string, normalized []string) error {
		_, headErr := c.run(ctx, "inspect HEAD", root, root, true, "rev-parse", "--verify", "--quiet", "HEAD")
		if headErr == nil {
			args := append([]string{literalPathspecs, "reset", "-q", "HEAD", "--"}, normalized...)
			_, err := c.run(ctx, "unstage", root, root, false, args...)
			return err
		}
		var gitErr *Error
		if !errors.As(headErr, &gitErr) || gitErr.Kind != CommandFailed {
			return headErr
		}
		args := append([]string{literalPathspecs, "rm", "--cached", "--ignore-unmatch", "-q", "--"}, normalized...)
		_, err := c.run(ctx, "unstage", root, root, false, args...)
		return err
	})
}

// Discard restores only explicitly supplied tracked paths from the index. It
// never removes untracked files.
func (c *Client) Discard(ctx context.Context, path string, paths ...string) error {
	return c.mutate(ctx, "discard", path, paths, func(root string, normalized []string) error {
		args := append([]string{literalPathspecs, "restore", "--worktree", "--"}, normalized...)
		_, err := c.run(ctx, "discard", root, root, false, args...)
		return err
	})
}

func (c *Client) mutate(
	ctx context.Context,
	operation, path string,
	paths []string,
	fn func(string, []string) error,
) error {
	if len(paths) == 0 {
		return newError(operation, path, InvalidPath, errors.New("at least one explicit path is required"), "")
	}
	root, err := c.Root(ctx, path)
	if err != nil {
		return err
	}
	normalized, err := normalizePaths(root, paths, false)
	if err != nil {
		return err
	}
	return fn(root, normalized)
}

func existingDirectory(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	current := filepath.Clean(abs)
	for {
		info, statErr := os.Stat(current)
		if statErr == nil {
			if info.IsDir() {
				return current, nil
			}
			return filepath.Dir(current), nil
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", statErr
		}
		current = parent
	}
}

func normalizePaths(root string, paths []string, allowRoot bool) ([]string, error) {
	normalized := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			return nil, newError("validate path", path, InvalidPath, errors.New("path is empty"), "")
		}
		candidate := path
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(root, filepath.FromSlash(candidate))
		}
		abs, err := filepath.Abs(candidate)
		if err != nil {
			return nil, newError("validate path", path, InvalidPath, err, "")
		}
		rel, err := filepath.Rel(root, filepath.Clean(abs))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			if err == nil {
				err = errors.New("path is outside the repository")
			}
			return nil, newError("validate path", path, InvalidPath, err, "")
		}
		if rel == "." && !allowRoot {
			err = errors.New("repository-wide mutation is not allowed")
			return nil, newError("validate path", path, InvalidPath, err, "")
		}
		normalized = append(normalized, filepath.ToSlash(rel))
	}
	return normalized, nil
}
