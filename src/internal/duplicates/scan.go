package duplicates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
)

var errStopFileLimit = errors.New("stop after maximum file count")

const (
	minimumGroupSize = 2
	hashBufferSize   = 128 * 1024
)

type candidate struct {
	path string
	info os.FileInfo
}

type scanner struct {
	ctx       context.Context
	options   Options
	result    Result
	files     []candidate
	seenFiles []os.FileInfo
	seenDirs  []os.FileInfo
}

type hashOutcome struct {
	digest string
	bytes  int64
	warn   *Warning
	done   bool
}

type groupKey struct {
	size   int64
	digest string
}

// Scan recursively discovers regular files below roots and returns groups of
// byte-for-byte duplicates. It only reads metadata and file content.
func Scan(ctx context.Context, roots []string, options Options) (Result, error) {
	if err := validateOptions(options); err != nil {
		return Result{}, err
	}
	if len(roots) == 0 {
		return Result{}, &ScanError{Code: ErrorInvalidRoots, Err: errors.New("at least one root is required")}
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	normalized, err := normalizeRoots(roots)
	if err != nil {
		return Result{}, err
	}
	s := scanner{ctx: ctx, options: options}
	for _, root := range normalized {
		if err := ctx.Err(); err != nil {
			return s.finish(), err
		}
		walkErr := s.walk(root, true)
		if errors.Is(walkErr, errStopFileLimit) {
			break
		}
		if walkErr != nil {
			return s.finish(), walkErr
		}
	}
	if err := ctx.Err(); err != nil {
		return s.finish(), err
	}

	s.hashCandidates()
	result := s.finish()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}

func validateOptions(options Options) error {
	if options.MaxFiles < 0 {
		return &ScanError{Code: ErrorInvalidOptions, Err: errors.New("max files cannot be negative")}
	}
	if options.MaxBytesHashed < 0 {
		return &ScanError{Code: ErrorInvalidOptions, Err: errors.New("max bytes hashed cannot be negative")}
	}
	if options.Workers < 0 {
		return &ScanError{Code: ErrorInvalidOptions, Err: errors.New("workers cannot be negative")}
	}
	return nil
}

func normalizeRoots(roots []string) ([]string, error) {
	normalized := make([]string, 0, len(roots))
	for _, root := range roots {
		if root == "" {
			return nil, &ScanError{Code: ErrorInvalidRoot, Err: errors.New("root cannot be empty")}
		}
		absolute, err := filepath.Abs(root)
		if err != nil {
			return nil, &ScanError{Code: ErrorInvalidRoot, Path: root, Err: err}
		}
		absolute = filepath.Clean(absolute)
		if _, err := os.Lstat(absolute); err != nil {
			return nil, &ScanError{Code: ErrorInvalidRoot, Path: absolute, Err: err}
		}
		normalized = append(normalized, absolute)
	}
	sort.Strings(normalized)
	return slices.Compact(normalized), nil
}

func (s *scanner) walk(path string, explicitRoot bool) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	info, skip := s.inspect(path, explicitRoot)
	if skip {
		return nil
	}
	if info.IsDir() {
		return s.walkDirectory(path, info)
	}
	return s.addFile(path, info)
}

func (s *scanner) inspect(path string, explicitRoot bool) (os.FileInfo, bool) {
	lstat, err := os.Lstat(path)
	if err != nil {
		s.warn(path, OperationStat, err)
		return nil, true
	}
	if !explicitRoot && !s.options.IncludeHidden && isHidden(path, lstat) {
		return nil, true
	}
	if lstat.Mode()&os.ModeSymlink == 0 {
		return lstat, false
	}
	if !s.options.FollowSymlinks {
		return nil, true
	}
	info, err := os.Stat(path)
	if err != nil {
		s.warn(path, OperationStat, err)
		return nil, true
	}
	return info, false
}

func (s *scanner) walkDirectory(path string, info os.FileInfo) error {
	if containsIdentity(s.seenDirs, info) {
		return nil
	}
	s.seenDirs = append(s.seenDirs, info)
	entries, err := os.ReadDir(path)
	if err != nil {
		s.warn(path, OperationReadDir, err)
		return nil
	}
	for _, entry := range entries {
		if err := s.walk(filepath.Join(path, entry.Name()), false); err != nil {
			return err
		}
	}
	return nil
}

func (s *scanner) addFile(path string, info os.FileInfo) error {
	if !info.Mode().IsRegular() {
		return nil
	}
	if !s.options.IncludeHardLinks && containsIdentity(s.seenFiles, info) {
		return nil
	}
	if s.options.MaxFiles > 0 && len(s.files) >= s.options.MaxFiles {
		s.result.Truncated = true
		s.warn(path, OperationLimit, ErrMaxFilesReached)
		return errStopFileLimit
	}

	s.seenFiles = append(s.seenFiles, info)
	s.files = append(s.files, candidate{path: path, info: info})
	s.result.FilesScanned++
	emit(s.options.Progress, Progress{Phase: PhaseDiscovered, Path: path, Files: s.result.FilesScanned})
	return nil
}

func containsIdentity(seen []os.FileInfo, info os.FileInfo) bool {
	for _, previous := range seen {
		if os.SameFile(previous, info) {
			return true
		}
	}
	return false
}

func (s *scanner) hashCandidates() {
	jobs := s.prepareHashJobs()
	if len(jobs) == 0 {
		return
	}
	outcomes := s.hashJobs(jobs)
	grouped := s.collectHashOutcomes(jobs, outcomes)
	s.appendDuplicateGroups(grouped)
}

func (s *scanner) prepareHashJobs() []candidate {
	buckets := make(map[int64][]candidate)
	for _, file := range s.files {
		buckets[file.info.Size()] = append(buckets[file.info.Size()], file)
	}

	jobs := make([]candidate, 0, len(s.files))
	sizes := make([]int64, 0, len(buckets))
	for size, files := range buckets {
		if len(files) >= minimumGroupSize {
			sizes = append(sizes, size)
		}
	}
	slices.Sort(sizes)
	var reserved int64
	for _, size := range sizes {
		files := buckets[size]
		slices.SortFunc(files, func(a, b candidate) int { return compareString(a.path, b.path) })
		for _, file := range files {
			if s.options.MaxBytesHashed > 0 && size > s.options.MaxBytesHashed-reserved {
				s.result.Truncated = true
				s.warn(file.path, OperationLimit, ErrHashBudgetReached)
				continue
			}
			reserved += size
			jobs = append(jobs, file)
		}
	}
	return jobs
}

func (s *scanner) hashJobs(jobs []candidate) []hashOutcome {
	workers := s.options.Workers
	if workers == 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	workers = min(workers, len(jobs))
	outcomes := make([]hashOutcome, len(jobs))
	var next atomic.Int64
	var wait sync.WaitGroup
	wait.Add(workers)
	for range workers {
		go hashWorker(s.ctx, jobs, outcomes, &next, &wait)
	}
	wait.Wait()
	return outcomes
}

func hashWorker(
	ctx context.Context,
	jobs []candidate,
	outcomes []hashOutcome,
	next *atomic.Int64,
	wait *sync.WaitGroup,
) {
	defer wait.Done()
	for {
		index := int(next.Add(1) - 1)
		if index >= len(jobs) {
			return
		}
		outcomes[index] = hashFile(ctx, jobs[index])
	}
}

func (s *scanner) collectHashOutcomes(jobs []candidate, outcomes []hashOutcome) map[groupKey][]File {
	grouped := make(map[groupKey][]File)
	for index, outcome := range outcomes {
		file := jobs[index]
		s.result.BytesHashed += outcome.bytes
		if outcome.warn != nil {
			s.result.Warnings = append(s.result.Warnings, *outcome.warn)
			continue
		}
		if !outcome.done {
			continue
		}
		s.result.FilesHashed++
		emit(s.options.Progress, Progress{
			Phase: PhaseHashed, Path: file.path, Files: s.result.FilesHashed, BytesHashed: s.result.BytesHashed,
		})
		key := groupKey{size: file.info.Size(), digest: outcome.digest}
		grouped[key] = append(grouped[key], File{Path: file.path, Size: key.size, Digest: key.digest})
	}
	return grouped
}

func (s *scanner) appendDuplicateGroups(grouped map[groupKey][]File) {
	for key, files := range grouped {
		if len(files) < minimumGroupSize {
			continue
		}
		slices.SortFunc(files, func(a, b File) int { return compareString(a.Path, b.Path) })
		s.result.Groups = append(s.result.Groups, Group{Size: key.size, Digest: key.digest, Files: files})
	}
}

func hashFile(ctx context.Context, file candidate) hashOutcome {
	if ctx.Err() != nil {
		return hashOutcome{}
	}
	handle, err := os.Open(file.path)
	if err != nil {
		warning := Warning{Path: file.path, Operation: OperationOpen, Err: err}
		return hashOutcome{warn: &warning}
	}
	defer handle.Close()

	before, err := handle.Stat()
	if err != nil {
		warning := Warning{Path: file.path, Operation: OperationStat, Err: err}
		return hashOutcome{warn: &warning}
	}
	if !sameSnapshot(file.info, before) {
		warning := Warning{Path: file.path, Operation: OperationRead, Err: errors.New("file changed before hashing")}
		return hashOutcome{warn: &warning}
	}

	hasher := sha256.New()
	buffer := make([]byte, hashBufferSize)
	remaining := file.info.Size()
	var read int64
	for remaining > 0 {
		if ctx.Err() != nil {
			return hashOutcome{bytes: read}
		}
		chunk := int64(len(buffer))
		if remaining < chunk {
			chunk = remaining
		}
		count, readErr := handle.Read(buffer[:chunk])
		if count > 0 {
			_, _ = hasher.Write(buffer[:count])
			read += int64(count)
			remaining -= int64(count)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				readErr = io.ErrUnexpectedEOF
			}
			warning := Warning{Path: file.path, Operation: OperationRead, Err: readErr}
			return hashOutcome{bytes: read, warn: &warning}
		}
		if count == 0 {
			warning := Warning{Path: file.path, Operation: OperationRead, Err: io.ErrNoProgress}
			return hashOutcome{bytes: read, warn: &warning}
		}
	}
	after, err := handle.Stat()
	if err != nil {
		warning := Warning{Path: file.path, Operation: OperationStat, Err: err}
		return hashOutcome{bytes: read, warn: &warning}
	}
	if !sameSnapshot(file.info, after) {
		warning := Warning{Path: file.path, Operation: OperationRead, Err: errors.New("file changed while hashing")}
		return hashOutcome{bytes: read, warn: &warning}
	}
	return hashOutcome{digest: hex.EncodeToString(hasher.Sum(nil)), bytes: read, done: true}
}

func sameSnapshot(expected, actual os.FileInfo) bool {
	return actual.Mode().IsRegular() && expected.Size() == actual.Size() &&
		expected.ModTime().Equal(actual.ModTime()) && os.SameFile(expected, actual)
}

func (s *scanner) warn(path string, operation Operation, err error) {
	s.result.Warnings = append(s.result.Warnings, Warning{Path: path, Operation: operation, Err: err})
}

func (s *scanner) finish() Result {
	slices.SortFunc(s.result.Groups, func(a, b Group) int {
		if a.Size < b.Size {
			return -1
		}
		if a.Size > b.Size {
			return 1
		}
		if digestOrder := compareString(a.Digest, b.Digest); digestOrder != 0 {
			return digestOrder
		}
		return compareString(a.Files[0].Path, b.Files[0].Path)
	})
	slices.SortFunc(s.result.Warnings, func(a, b Warning) int {
		if pathOrder := compareString(a.Path, b.Path); pathOrder != 0 {
			return pathOrder
		}
		if operationOrder := compareString(string(a.Operation), string(b.Operation)); operationOrder != 0 {
			return operationOrder
		}
		return compareString(fmt.Sprint(a.Err), fmt.Sprint(b.Err))
	})
	return s.result
}

func compareString(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func emit(channel chan<- Progress, progress Progress) {
	if channel == nil {
		return
	}
	select {
	case channel <- progress:
	default:
	}
}
