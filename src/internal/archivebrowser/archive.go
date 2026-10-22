package archivebrowser

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"io/fs"
	"math"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	defaultDirectoryPerm fs.FileMode = 0o755
	defaultFilePerm      fs.FileMode = 0o644
	operationIndex                   = "index"
	operationList                    = "list"
	operationStat                    = "stat"
	operationRead                    = "read"
	operationExtract                 = "extract"
)

// Entry describes one normalized member of an archive. Path always uses '/'
// separators and is relative to the archive root.
type Entry struct {
	Path           string
	Name           string
	IsDir          bool
	Implicit       bool
	Size           uint64
	CompressedSize uint64
	Mode           fs.FileMode
	Modified       time.Time
}

type node struct {
	entry    Entry
	file     *zip.File
	children map[string]struct{}
}

// Archive is a read-only ZIP archive and its normalized directory index.
// Archive methods may be called concurrently with each other. Close waits for
// an in-progress operation to finish.
type Archive struct {
	mu     sync.RWMutex
	path   string
	reader *zip.ReadCloser
	nodes  map[string]*node
	closed bool
}

// Supported reports whether name has a .zip extension (case-insensitive).
func Supported(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".zip")
}

// Open opens a ZIP archive read-only and builds its normalized directory tree.
func Open(archivePath string) (*Archive, error) {
	if !Supported(archivePath) {
		return nil, &Error{Op: "open", Archive: archivePath, Err: ErrUnsupportedFormat}
	}
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, &Error{Op: "open", Archive: archivePath, Err: errors.Join(ErrInvalidArchive, err)}
	}
	archive := &Archive{
		path:   archivePath,
		reader: reader,
		nodes: map[string]*node{
			"": {
				entry:    Entry{IsDir: true, Mode: fs.ModeDir | defaultDirectoryPerm},
				children: make(map[string]struct{}),
			},
		},
	}
	for _, file := range reader.File {
		if err := archive.index(file); err != nil {
			_ = reader.Close()
			return nil, err
		}
	}
	return archive, nil
}

func (a *Archive) index(file *zip.File) error {
	normalized, directory, err := normalizePath(file.Name, false)
	if err != nil {
		return &Error{Op: operationIndex, Archive: a.path, Member: file.Name, Err: err}
	}
	parts := strings.Split(normalized, "/")
	for i := 1; i < len(parts); i++ {
		ancestor := strings.Join(parts[:i], "/")
		if existing := a.nodes[ancestor]; existing != nil {
			if !existing.entry.IsDir {
				return &Error{Op: operationIndex, Archive: a.path, Member: normalized, Err: ErrDuplicatePath}
			}
			continue
		}
		a.nodes[ancestor] = &node{
			entry: Entry{
				Path:     ancestor,
				Name:     path.Base(ancestor),
				IsDir:    true,
				Implicit: true,
				Mode:     fs.ModeDir | defaultDirectoryPerm,
			},
			children: make(map[string]struct{}),
		}
		a.addChild(ancestor)
	}

	mode := file.Mode()
	directory = directory || mode.IsDir()
	if existing := a.nodes[normalized]; existing != nil {
		if !directory || !existing.entry.IsDir || !existing.entry.Implicit {
			return &Error{Op: operationIndex, Archive: a.path, Member: normalized, Err: ErrDuplicatePath}
		}
		existing.entry = entryFromFile(file, normalized, true)
		existing.file = file
		return nil
	}
	a.nodes[normalized] = &node{
		entry:    entryFromFile(file, normalized, directory),
		file:     file,
		children: make(map[string]struct{}),
	}
	a.addChild(normalized)
	return nil
}

func entryFromFile(file *zip.File, normalized string, directory bool) Entry {
	mode := file.Mode()
	if directory {
		mode |= fs.ModeDir
	}
	return Entry{
		Path:           normalized,
		Name:           path.Base(normalized),
		IsDir:          directory,
		Size:           file.UncompressedSize64,
		CompressedSize: file.CompressedSize64,
		Mode:           mode,
		Modified:       file.Modified,
	}
}

func (a *Archive) addChild(member string) {
	parent := path.Dir(member)
	if parent == "." {
		parent = ""
	}
	a.nodes[parent].children[member] = struct{}{}
}

// Close closes the underlying ZIP file. It is safe to call more than once.
func (a *Archive) Close() error {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	a.closed = true
	if err := a.reader.Close(); err != nil {
		return &Error{Op: "close", Archive: a.path, Err: err}
	}
	return nil
}

// List returns the direct children of directory. Directories sort before files;
// each group is sorted by normalized path for deterministic results.
func (a *Archive) List(directory string) ([]Entry, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.closed {
		return nil, &Error{Op: operationList, Archive: a.path, Member: directory, Err: ErrClosed}
	}
	normalized, _, err := normalizePath(directory, true)
	if err != nil {
		return nil, &Error{Op: operationList, Archive: a.path, Member: directory, Err: err}
	}
	n := a.nodes[normalized]
	if n == nil {
		return nil, &Error{Op: operationList, Archive: a.path, Member: normalized, Err: fs.ErrNotExist}
	}
	if !n.entry.IsDir {
		return nil, &Error{Op: operationList, Archive: a.path, Member: normalized, Err: ErrNotDirectory}
	}
	entries := make([]Entry, 0, len(n.children))
	for child := range n.children {
		entries = append(entries, a.nodes[child].entry)
	}
	slices.SortFunc(entries, func(left, right Entry) int {
		if left.IsDir != right.IsDir {
			if left.IsDir {
				return -1
			}
			return 1
		}
		return strings.Compare(left.Path, right.Path)
	})
	return entries, nil
}

// Stat returns metadata for one member. An empty path addresses the root.
func (a *Archive) Stat(member string) (Entry, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.closed {
		return Entry{}, &Error{Op: operationStat, Archive: a.path, Member: member, Err: ErrClosed}
	}
	normalized, _, err := normalizePath(member, true)
	if err != nil {
		return Entry{}, &Error{Op: operationStat, Archive: a.path, Member: member, Err: err}
	}
	n := a.nodes[normalized]
	if n == nil {
		return Entry{}, &Error{Op: operationStat, Archive: a.path, Member: normalized, Err: fs.ErrNotExist}
	}
	return n.entry, nil
}

// Read returns one file member, failing if more than maxBytes would be returned.
// A zero limit permits only an empty file.
func (a *Archive) Read(ctx context.Context, member string, maxBytes int64) ([]byte, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.closed {
		return nil, &Error{Op: operationRead, Archive: a.path, Member: member, Err: ErrClosed}
	}
	if ctx == nil {
		return nil, &Error{
			Op:      operationRead,
			Archive: a.path,
			Member:  member,
			Err:     errors.Join(ErrInvalidArgument, errors.New("nil context")),
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, &Error{Op: operationRead, Archive: a.path, Member: member, Err: err}
	}
	if maxBytes < 0 {
		return nil, &Error{Op: operationRead, Archive: a.path, Member: member, Err: ErrInvalidLimit}
	}
	normalized, _, err := normalizePath(member, false)
	if err != nil {
		return nil, &Error{Op: operationRead, Archive: a.path, Member: member, Err: err}
	}
	n := a.nodes[normalized]
	if n == nil {
		return nil, &Error{Op: operationRead, Archive: a.path, Member: normalized, Err: fs.ErrNotExist}
	}
	if n.entry.IsDir {
		return nil, &Error{Op: operationRead, Archive: a.path, Member: normalized, Err: ErrIsDirectory}
	}
	if n.entry.Size > uint64(maxBytes) {
		return nil, &Error{Op: operationRead, Archive: a.path, Member: normalized, Err: ErrLimitExceeded}
	}
	reader, err := n.file.Open()
	if err != nil {
		return nil, &Error{Op: operationRead, Archive: a.path, Member: normalized, Err: err}
	}
	defer reader.Close()

	var source io.Reader = &contextReader{ctx: ctx, reader: reader}
	if maxBytes < math.MaxInt64 {
		source = io.LimitReader(source, maxBytes+1)
	}
	data, err := io.ReadAll(source)
	if err != nil {
		if errors.Is(err, zip.ErrChecksum) || errors.Is(err, io.ErrUnexpectedEOF) {
			err = errors.Join(ErrInvalidArchive, err)
		}
		return nil, &Error{Op: operationRead, Archive: a.path, Member: normalized, Err: err}
	}
	if int64(len(data)) > maxBytes {
		return nil, &Error{Op: operationRead, Archive: a.path, Member: normalized, Err: ErrLimitExceeded}
	}
	return data, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

func normalizePath(name string, allowRoot bool) (string, bool, error) {
	if strings.IndexByte(name, 0) >= 0 {
		return "", false, ErrUnsafePath
	}
	name = strings.ReplaceAll(name, `\`, "/")
	if allowRoot && (name == "" || name == ".") {
		return "", true, nil
	}
	if name == "" || strings.HasPrefix(name, "/") || hasDrivePrefix(name) {
		return "", false, ErrUnsafePath
	}
	directory := strings.HasSuffix(name, "/")
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", false, ErrUnsafePath
		}
	}
	normalized := path.Clean(name)
	if normalized == "." || normalized == ".." || strings.HasPrefix(normalized, "../") {
		return "", false, ErrUnsafePath
	}
	return normalized, directory, nil
}

func hasDrivePrefix(name string) bool {
	if len(name) < 2 || name[1] != ':' {
		return false
	}
	first := name[0]
	return (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')
}
