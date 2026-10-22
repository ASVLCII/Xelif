// Package remote provides an explicit, read-only-by-default rclone adapter for
// discovering configured cloud remotes, listing directories, and downloading a
// selected file. It never stores or requests credentials.
package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"slices"
	"strings"
	"time"
)

const (
	defaultTimeout        = 30 * time.Second
	defaultMaxOutputBytes = int64(4 << 20)
	maxStderrBytes        = int64(64 << 10)
	nullJSON              = "null"
)

var errOutputTooLarge = errors.New("rclone output exceeded the configured limit")

// Options bounds each rclone invocation. Zero fields select safe defaults.
type Options struct {
	Timeout        time.Duration
	MaxOutputBytes int64
}

// Client invokes the externally installed rclone executable. Construction
// performs only executable discovery and never contacts a remote.
type Client struct {
	binary    string
	runner    commandRunner
	timeout   time.Duration
	maxOutput int64
}

// Remote describes a configured rclone remote without exposing its settings or
// credentials.
type Remote struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

// Entry is a normalized direct child returned by List. Path is relative to the
// remote root and always uses slash separators.
type Entry struct {
	Remote  string
	Path    string
	Name    string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

// New discovers rclone with exec.LookPath. At most one Options value may be
// supplied.
func New(options ...Options) (*Client, error) {
	if len(options) > 1 {
		return nil, errors.New("remote: multiple Options values")
	}
	var opts Options
	if len(options) == 1 {
		opts = options[0]
	}
	return newClient(exec.LookPath, execRunner{}, opts)
}

func newClient(lookPath func(string) (string, error), runner commandRunner, options Options) (*Client, error) {
	if options.Timeout < 0 {
		return nil, errors.New("remote: Timeout must not be negative")
	}
	if options.MaxOutputBytes < 0 {
		return nil, errors.New("remote: MaxOutputBytes must not be negative")
	}
	if options.Timeout == 0 {
		options.Timeout = defaultTimeout
	}
	if options.MaxOutputBytes == 0 {
		options.MaxOutputBytes = defaultMaxOutputBytes
	}
	binary, err := lookPath("rclone")
	if err != nil {
		return nil, newError("discover", "", Unavailable, err, "")
	}
	return &Client{binary: binary, runner: runner, timeout: options.Timeout, maxOutput: options.MaxOutputBytes}, nil
}

// Remotes returns configured remotes in deterministic name order.
func (c *Client) Remotes(ctx context.Context) ([]Remote, error) {
	output, err := c.run(ctx, "remotes", "", "listremotes", "--json", "--ask-password=false")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(output)) == nullJSON {
		return nil, newError("remotes", "", InvalidOutput, errors.New("rclone returned null JSON"), "")
	}
	var remotes []Remote
	if err := json.Unmarshal(output, &remotes); err != nil {
		return nil, newError("remotes", "", InvalidOutput, fmt.Errorf("decode JSON: %w", err), "")
	}
	seen := make(map[string]struct{}, len(remotes))
	for _, remote := range remotes {
		if err := validateRemoteName(remote.Name); err != nil || strings.TrimSpace(remote.Type) == "" {
			return nil, newError("remotes", remote.Name+":", InvalidOutput, errors.New("invalid remote metadata"), "")
		}
		if _, duplicate := seen[remote.Name]; duplicate {
			return nil, newError("remotes", remote.Name+":", InvalidOutput, errors.New("duplicate remote name"), "")
		}
		seen[remote.Name] = struct{}{}
	}
	slices.SortFunc(remotes, func(a, b Remote) int { return strings.Compare(a.Name, b.Name) })
	return remotes, nil
}

type jsonEntry struct {
	Path    string    `json:"Path"`
	Name    string    `json:"Name"`
	Size    int64     `json:"Size"`
	ModTime time.Time `json:"ModTime"`
	IsDir   bool      `json:"IsDir"`
}

// List returns the direct children of location. Calling it is the explicit
// point at which network access to a configured remote may occur.
func (c *Client) List(ctx context.Context, location RemotePath) ([]Entry, error) {
	rawLocation := location.String()
	location, err := normalizeRemotePath(location, true)
	if err != nil {
		return nil, newError("list", rawLocation, InvalidPath, err, "")
	}
	ctx, cancel := c.operationContext(ctx)
	defer cancel()
	if configureErr := c.ensureConfigured(ctx, location.Remote); configureErr != nil {
		return nil, configureErr
	}
	output, err := c.run(
		ctx,
		"list",
		location.String(),
		"lsjson",
		location.String(),
		"--no-mimetype",
		"--ask-password=false",
	)
	if err != nil {
		return nil, err
	}
	return decodeEntries(location, output)
}

func decodeEntries(location RemotePath, output []byte) ([]Entry, error) {
	if strings.TrimSpace(string(output)) == nullJSON {
		return nil, newError("list", location.String(), InvalidOutput, errors.New("rclone returned null JSON"), "")
	}
	var raw []jsonEntry
	if decodeErr := json.Unmarshal(output, &raw); decodeErr != nil {
		return nil, newError("list", location.String(), InvalidOutput, fmt.Errorf("decode JSON: %w", decodeErr), "")
	}
	entries := make([]Entry, 0, len(raw))
	for _, item := range raw {
		entry, entryErr := normalizeEntry(location, item)
		if entryErr != nil {
			return nil, entryErr
		}
		entries = append(entries, entry)
	}
	slices.SortFunc(entries, compareEntries)
	return entries, nil
}

func normalizeEntry(location RemotePath, item jsonEntry) (Entry, error) {
	child, err := normalizeObjectPath(item.Path, false)
	if err != nil || item.Name == "" || path.Base(child) != item.Name {
		return Entry{}, newError(
			"list",
			location.String(),
			InvalidOutput,
			errors.New("rclone returned an invalid entry path"),
			"",
		)
	}
	fullPath := child
	if location.Path != "" {
		fullPath = path.Join(location.Path, child)
	}
	fullPath, err = normalizeObjectPath(fullPath, false)
	if err != nil || (!item.IsDir && item.Size < 0) {
		return Entry{}, newError(
			"list",
			location.String(),
			InvalidOutput,
			errors.New("rclone returned invalid entry metadata"),
			"",
		)
	}
	if item.IsDir {
		item.Size = 0
	}
	return Entry{
		Remote:  location.Remote,
		Path:    fullPath,
		Name:    item.Name,
		Size:    item.Size,
		ModTime: item.ModTime.UTC(),
		IsDir:   item.IsDir,
	}, nil
}

func compareEntries(a, b Entry) int {
	if compared := strings.Compare(a.Path, b.Path); compared != 0 {
		return compared
	}
	if a.IsDir == b.IsDir {
		return 0
	}
	if a.IsDir {
		return -1
	}
	return 1
}

func (c *Client) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, c.timeout)
}

func (c *Client) ensureConfigured(ctx context.Context, name string) error {
	remotes, err := c.Remotes(ctx)
	if err != nil {
		return err
	}
	for _, remote := range remotes {
		if remote.Name == name {
			return nil
		}
	}
	return newError("configure", name+":", NotConfigured, errors.New("rclone remote is not configured"), "")
}

func (c *Client) run(ctx context.Context, operation, remotePath string, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	bounded, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	result := c.runner.Run(bounded, c.binary, args, c.maxOutput, maxStderrBytes)
	stderr := string(result.stderr)
	if result.outputExceeded {
		return nil, newError(operation, remotePath, OutputLimit, errOutputTooLarge, stderr)
	}
	if err := bounded.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, newError(operation, remotePath, TimedOut, err, stderr)
		}
		return nil, newError(operation, remotePath, Cancelled, err, stderr)
	}
	if result.err == nil {
		return result.stdout, nil
	}
	if errors.Is(result.err, context.DeadlineExceeded) {
		return nil, newError(operation, remotePath, TimedOut, result.err, stderr)
	}
	if errors.Is(result.err, context.Canceled) {
		return nil, newError(operation, remotePath, Cancelled, result.err, stderr)
	}
	var execErr *exec.Error
	if errors.As(result.err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		return nil, newError(operation, remotePath, Unavailable, result.err, stderr)
	}
	kind := classifyCommandError(stderr)
	return nil, newError(operation, remotePath, kind, result.err, stderr)
}

func classifyCommandError(stderr string) ErrorKind {
	message := strings.ToLower(stderr)
	for _, phrase := range []string{
		"didn't find section in config file", "not found in config file", "remote is not configured",
	} {
		if strings.Contains(message, phrase) {
			return NotConfigured
		}
	}
	for _, phrase := range []string{
		"unauthorized", "authentication", "invalid credential", "access denied", "permission denied",
		"token expired", "invalid token", "oauth", "http error 401", "http error 403",
	} {
		if strings.Contains(message, phrase) {
			return Authentication
		}
	}
	return CommandFailed
}
