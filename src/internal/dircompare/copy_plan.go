package dircompare

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"slices"
)

// copyApproval is never exposed for callers to mutate. In particular, actions
// has its own backing array, independent of the displayable CopyResult.Actions.
type copyApproval struct {
	sourceAbs       string
	destinationAbs  string
	sourceRoot      os.FileInfo
	destinationRoot os.FileInfo
	actions         []CopyAction
	sources         map[string]sourceFingerprint
}

type sourceFingerprint struct {
	info os.FileInfo
	hash [sha256.Size]byte
}

func freezeCopyPlan(ctx context.Context, plan copyPlan) (*copyApproval, error) {
	sourceRoot, err := captureCopyIdentity(plan.sourceAbs)
	if err != nil {
		return nil, err
	}
	destinationRoot, err := captureCopyIdentity(plan.destinationAbs)
	if err != nil {
		return nil, err
	}
	approval := &copyApproval{
		sourceAbs:       plan.sourceAbs,
		destinationAbs:  plan.destinationAbs,
		sourceRoot:      sourceRoot,
		destinationRoot: destinationRoot,
		actions: slices.Clone(
			plan.result.Actions,
		),
		sources: make(map[string]sourceFingerprint, len(plan.result.Actions)),
	}
	for _, action := range approval.actions {
		fingerprint, err := fingerprintCopySource(ctx, plan.source[action.Path])
		if err != nil {
			return nil, &PathError{Op: "snapshot preview source", Path: action.Path, Err: err}
		}
		approval.sources[action.Path] = fingerprint
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return approval, nil
}

func (p *copyPlan) validateApproval(ctx context.Context, preview CopyResult) error {
	approval := preview.approval
	if approval == nil || !slices.Equal(preview.Actions, approval.actions) {
		return &StalePlanError{Err: errors.New("a successful, unchanged copy-missing preview is required")}
	}
	if p.sourceAbs != approval.sourceAbs || p.destinationAbs != approval.destinationAbs {
		return &StalePlanError{Err: errors.New("preview roots changed")}
	}
	if err := approval.validateRoots(); err != nil {
		return err
	}
	if !slices.Equal(p.result.Actions, approval.actions) {
		return &StalePlanError{Err: errors.New("the set of missing items changed")}
	}
	for _, action := range approval.actions {
		current, err := fingerprintCopySource(ctx, p.source[action.Path])
		if err != nil {
			return &StalePlanError{Path: action.Path, Err: err}
		}
		expected := approval.sources[action.Path]
		if !sameCopySourceInfo(expected.info, current.info) || expected.hash != current.hash {
			return &StalePlanError{Path: action.Path, Err: errors.New("source changed since preview")}
		}
	}
	// Execute only the frozen actions, not a newly expanded traversal result.
	p.result.Actions = slices.Clone(approval.actions)
	p.approval = approval
	return nil
}

func (a *copyApproval) validateRoots() error {
	for _, root := range []struct {
		path string
		info os.FileInfo
	}{{a.sourceAbs, a.sourceRoot}, {a.destinationAbs, a.destinationRoot}} {
		current, err := os.Lstat(root.path)
		if err != nil {
			return &StalePlanError{Err: err}
		}
		if !current.IsDir() || !os.SameFile(root.info, current) {
			return &StalePlanError{Err: errors.New("a preview root was replaced")}
		}
	}
	return nil
}

func (e *copyExecutor) validateApprovedAction(action CopyAction) error {
	if e.approval == nil {
		return nil
	}
	if err := e.approval.validateRoots(); err != nil {
		return err
	}
	if err := ensureSafeParents(e.sourceAbs, action.Path); err != nil {
		return &StalePlanError{Path: action.Path, Err: err}
	}
	if action.Kind == CreateDirectory {
		current, err := os.Lstat(e.source[action.Path].absPath)
		if err != nil {
			return &StalePlanError{Path: action.Path, Err: err}
		}
		if !sameCopySourceInfo(e.approval.sources[action.Path].info, current) {
			return &StalePlanError{Path: action.Path, Err: errors.New("source directory changed since preview")}
		}
	}
	return nil
}

func captureCopyIdentity(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	// Windows path-based FileInfo loads its file ID lazily. Force that capture
	// now, while this path still names the object shown in the preview.
	if !os.SameFile(info, info) {
		return nil, errors.New("could not capture filesystem identity")
	}
	return info, nil
}

func fingerprintCopySource(ctx context.Context, entry treeEntry) (sourceFingerprint, error) {
	if err := ctx.Err(); err != nil {
		return sourceFingerprint{}, err
	}
	info, err := captureCopyIdentity(entry.absPath)
	if err != nil {
		return sourceFingerprint{}, err
	}
	if !sameCopySourceMetadata(entry.metadata, info) {
		return sourceFingerprint{}, errors.New("source changed while planning")
	}
	fingerprint := sourceFingerprint{info: info}
	if entry.metadata.Type != TypeRegular {
		return fingerprint, nil
	}
	source, err := openStableSource(entry.absPath, entry.absPath)
	if err != nil {
		return sourceFingerprint{}, err
	}
	defer source.Close()
	if verifyErr := validateOpenApprovedSource(source, entry.absPath, entry.absPath, &fingerprint); verifyErr != nil {
		return sourceFingerprint{}, verifyErr
	}
	fingerprint.hash, err = hashOpenCopySource(ctx, source)
	if err != nil {
		return sourceFingerprint{}, err
	}
	if err := validateOpenApprovedSource(source, entry.absPath, entry.absPath, &fingerprint); err != nil {
		return sourceFingerprint{}, err
	}
	return fingerprint, nil
}

func sameCopySourceMetadata(expected Metadata, current os.FileInfo) bool {
	if expected.Type != metadataFromInfo(current).Type || expected.Mode != current.Mode() {
		return false
	}
	return !current.Mode().IsRegular() || (expected.Size == current.Size() && expected.ModTime.Equal(current.ModTime()))
}

func sameCopySourceInfo(expected, current os.FileInfo) bool {
	return os.SameFile(expected, current) && sameCopySourceMetadata(metadataFromInfo(expected), current)
}

func validateOpenApprovedSource(source *os.File, sourcePath, displayPath string, expected *sourceFingerprint) error {
	if expected == nil {
		return nil
	}
	opened, err := source.Stat()
	if err != nil {
		return &StalePlanError{Path: displayPath, Err: err}
	}
	current, err := os.Lstat(sourcePath)
	if err != nil {
		return &StalePlanError{Path: displayPath, Err: err}
	}
	if !sameCopySourceInfo(expected.info, opened) || !sameCopySourceInfo(expected.info, current) {
		return &StalePlanError{
			Path: displayPath,
			Err:  errors.New("source identity, type, or metadata changed since preview"),
		}
	}
	return nil
}

func hashOpenCopySource(ctx context.Context, source *os.File) ([sha256.Size]byte, error) {
	var result [sha256.Size]byte
	digest := sha256.New()
	buffer := make([]byte, fileBufferSize)
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		n, readErr := source.Read(buffer)
		if n > 0 {
			if _, err := digest.Write(buffer[:n]); err != nil {
				return result, err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return result, readErr
		}
	}
	copy(result[:], digest.Sum(nil))
	return result, nil
}
