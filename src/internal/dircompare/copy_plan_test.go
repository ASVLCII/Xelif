package dircompare

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestCopyMissingApprovedPlanRejectsChangedSource(t *testing.T) {
	for _, scenario := range []string{"added", "removed", "modified", "replaced", "type", "directory-replaced"} {
		t.Run(scenario, func(t *testing.T) {
			source, destination := t.TempDir(), t.TempDir()
			mustMkdir(t, filepath.Join(source, "folder"), 0o750)
			path := filepath.Join(source, "reviewed.txt")
			mustWrite(t, path, "reviewed", time.Now(), 0o600)
			plan := previewCopy(t, source, destination)
			mutateCopyPlanSource(t, source, path, scenario)
			result, err := CopyMissing(t.Context(), source, destination, CopyOptions{ApprovedPlan: &plan})
			var stale *StalePlanError
			if !errors.As(err, &stale) || len(result.Created) != 0 {
				t.Fatalf("result=%+v error=%v, want stale preview before any creation", result, err)
			}
			entries, readErr := os.ReadDir(destination)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("destination changed: entries=%v error=%v", entries, readErr)
			}
		})
	}
}

func mutateCopyPlanSource(t *testing.T, source, path, scenario string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	switch scenario {
	case "added":
		mustWrite(t, filepath.Join(source, "unreviewed.txt"), "new", time.Now(), 0o600)
	case "removed":
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	case "modified":
		mustWrite(t, path, "modified", info.ModTime(), info.Mode().Perm())
	case "replaced", "type":
		// Keep the original inode alive so this identity regression does not
		// depend on a filesystem choosing not to reuse a deleted file's ID.
		if err := os.Rename(path, filepath.Join(t.TempDir(), "original")); err != nil {
			t.Fatal(err)
		}
		if scenario == "type" {
			mustMkdir(t, path, 0o700)
		} else {
			mustWrite(t, path, "reviewed", info.ModTime(), info.Mode().Perm())
		}
	case "directory-replaced":
		folder := filepath.Join(source, "folder")
		if err := os.Rename(folder, filepath.Join(t.TempDir(), "original-folder")); err != nil {
			t.Fatal(err)
		}
		mustMkdir(t, folder, 0o750)
	}
}

func TestCopyMissingApprovedPlanPreservesDestinationArrival(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "directory"}[directory], func(t *testing.T) {
			source, destination := t.TempDir(), t.TempDir()
			mustWrite(t, filepath.Join(source, "reviewed.txt"), "reviewed", time.Now(), 0o600)
			plan := previewCopy(t, source, destination)
			path := filepath.Join(destination, "reviewed.txt")
			if directory {
				mustMkdir(t, path, 0o700)
				mustWrite(t, filepath.Join(path, "external.txt"), "keep", time.Now(), 0o600)
			} else {
				mustWrite(t, path, "keep", time.Now(), 0o600)
			}
			result, err := CopyMissing(t.Context(), source, destination, CopyOptions{ApprovedPlan: &plan})
			var stale *StalePlanError
			if !errors.As(err, &stale) || len(result.Created) != 0 {
				t.Fatalf("result=%+v error=%v, want stale preview before any creation", result, err)
			}
			if directory {
				path = filepath.Join(path, "external.txt")
			}
			assertCopyContents(t, path, "keep")
		})
	}
}

func TestCopyMissingApprovedPlanRejectsRootReplacement(t *testing.T) {
	for _, replaceSource := range []bool{false, true} {
		t.Run(map[bool]string{false: "destination", true: "source"}[replaceSource], func(t *testing.T) {
			source, destination := t.TempDir(), t.TempDir()
			mustWrite(t, filepath.Join(source, "reviewed.txt"), "reviewed", time.Now(), 0o600)
			plan := previewCopy(t, source, destination)
			path := destination
			if replaceSource {
				path = source
			}
			if err := os.Rename(path, filepath.Join(t.TempDir(), "original")); err != nil {
				t.Fatal(err)
			}
			mustMkdir(t, path, 0o700)
			if replaceSource {
				mustWrite(t, filepath.Join(source, "reviewed.txt"), "reviewed", time.Now(), 0o600)
			}
			_, err := CopyMissing(t.Context(), source, destination, CopyOptions{ApprovedPlan: &plan})
			var stale *StalePlanError
			if !errors.As(err, &stale) {
				t.Fatalf("error=%v, want stale replaced root", err)
			}
			assertCopyAbsent(t, filepath.Join(destination, "reviewed.txt"))
		})
	}
}

func TestCopyMissingApprovedPlanDoesNotExpandDuringApply(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	mustWrite(t, filepath.Join(source, "reviewed.txt"), "reviewed", time.Now(), 0o600)
	mustWrite(t, filepath.Join(source, "existing.txt"), "source", time.Now(), 0o600)
	mustWrite(t, filepath.Join(destination, "existing.txt"), "keep", time.Now(), 0o600)
	plan := previewCopy(t, source, destination)
	result, err := CopyMissing(t.Context(), source, destination, CopyOptions{
		ApprovedPlan: &plan,
		Progress: func(Progress) {
			mustWrite(t, filepath.Join(source, "unreviewed.txt"), "new", time.Now(), 0o600)
		},
	})
	if err != nil || !slices.Equal(result.Created, []string{"reviewed.txt"}) {
		t.Fatalf("result=%+v error=%v, want only reviewed file", result, err)
	}
	assertCopyAbsent(t, filepath.Join(destination, "unreviewed.txt"))
	assertCopyContents(t, filepath.Join(destination, "reviewed.txt"), "reviewed")
	assertCopyContents(t, filepath.Join(destination, "existing.txt"), "keep")
}

func TestCopyMissingApprovedPlanRevalidatesEachSourceAndRetainsPartialCopy(t *testing.T) {
	for _, scenario := range []string{"modified", "replaced", "collision", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			source, destination := t.TempDir(), t.TempDir()
			mustWrite(t, filepath.Join(source, "a.txt"), "first", time.Now(), 0o600)
			path := filepath.Join(source, "b.txt")
			mustWrite(t, path, "reviewed", time.Now(), 0o600)
			plan := previewCopy(t, source, destination)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result, err := CopyMissing(ctx, source, destination, CopyOptions{
				ApprovedPlan: &plan,
				Progress: func(Progress) {
					switch scenario {
					case "collision":
						mustWrite(t, filepath.Join(destination, "b.txt"), "keep", time.Now(), 0o600)
					case "cancelled":
						cancel()
					default:
						mutateCopyPlanSource(t, source, path, scenario)
					}
				},
			})
			assertPartialCopyError(t, err, []string{"a.txt"})
			assertApprovedCopyFailure(t, scenario, err)
			if !slices.Equal(result.Created, []string{"a.txt"}) {
				t.Fatalf("creation audit=%v", result.Created)
			}
			assertCopyContents(t, filepath.Join(destination, "a.txt"), "first")
			if scenario == "collision" {
				assertCopyContents(t, filepath.Join(destination, "b.txt"), "keep")
			} else {
				assertCopyAbsent(t, filepath.Join(destination, "b.txt"))
			}
		})
	}
}

func assertApprovedCopyFailure(t *testing.T, scenario string, err error) {
	t.Helper()
	switch scenario {
	case "modified", "replaced":
		var stale *StalePlanError
		if !errors.As(err, &stale) {
			t.Fatalf("error=%v, want stale source", err)
		}
	case "collision":
		var collision *CollisionError
		if !errors.As(err, &collision) {
			t.Fatalf("error=%v, want destination collision", err)
		}
	case "cancelled":
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v, want cancellation", err)
		}
	}
}

func TestCopyMissingApprovedPlanRejectsUntrustedApprovalAndCancellation(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	mustWrite(t, filepath.Join(source, "reviewed.txt"), "reviewed", time.Now(), 0o600)
	plan := previewCopy(t, source, destination)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := CopyMissing(ctx, source, destination, CopyOptions{ApprovedPlan: &plan})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v, want cancellation", err)
	}
	for _, approval := range []CopyResult{{Actions: plan.Actions}, plan} {
		approval.Actions = []CopyAction{{Path: "../unreviewed", Kind: CopyFile, Mode: 0o600}}
		_, err = CopyMissing(t.Context(), source, destination, CopyOptions{ApprovedPlan: &approval})
		var stale *StalePlanError
		if !errors.As(err, &stale) {
			t.Fatalf("error=%v, want stale untrusted approval", err)
		}
	}
	assertCopyAbsent(t, filepath.Join(destination, "reviewed.txt"))
}

func previewCopy(t *testing.T, source, destination string) CopyResult {
	t.Helper()
	result, err := CopyMissing(t.Context(), source, destination, CopyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
