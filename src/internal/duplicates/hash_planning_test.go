package duplicates

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Kept verbatim in behavior from aff5844 as a differential oracle and benchmark.
func prepareHashJobsAff5844(s *scanner) []candidate {
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

type planningFileInfo struct{ size int64 }

func (i planningFileInfo) Name() string       { return "file" }
func (i planningFileInfo) Size() int64        { return i.size }
func (i planningFileInfo) Mode() os.FileMode  { return 0o644 }
func (i planningFileInfo) ModTime() time.Time { return time.Time{} }
func (i planningFileInfo) IsDir() bool        { return false }
func (i planningFileInfo) Sys() any           { return nil }

func TestHashPlanningMatchesAff5844(t *testing.T) {
	for _, size := range []int64{0, 1, 32} {
		files := []candidate{
			{path: "z", info: planningFileInfo{size}},
			{path: "a", info: planningFileInfo{size}},
			{path: "m", info: planningFileInfo{size}},
		}
		for _, budget := range []int64{0, 1, 31, 32, 64} {
			compareHashPlanning(t, files, budget)
		}
	}
	random := rand.New(rand.NewPCG(1, 2))
	for range 200 {
		count := random.IntN(300)
		files := make([]candidate, count)
		for i := range files {
			files[i] = candidate{path: fmt.Sprintf("file-%04d", i), info: planningFileInfo{size: int64(random.IntN(30))}}
		}
		random.Shuffle(len(files), func(i, j int) { files[i], files[j] = files[j], files[i] })
		for _, budget := range []int64{0, 1, 31, 127, math.MaxInt64} {
			compareHashPlanning(t, files, budget)
		}
	}
	compareHashPlanning(t, []candidate{
		{path: "z", info: planningFileInfo{math.MaxInt64}},
		{path: "a", info: planningFileInfo{math.MaxInt64}},
		{path: "empty-z", info: planningFileInfo{0}},
		{path: "empty-a", info: planningFileInfo{0}},
		{path: "unique", info: planningFileInfo{12}},
	}, math.MaxInt64)
}

func compareHashPlanning(t *testing.T, files []candidate, budget int64) {
	t.Helper()
	reference := scanner{files: slices.Clone(files), options: Options{MaxBytesHashed: budget}}
	updated := scanner{files: slices.Clone(files), options: Options{MaxBytesHashed: budget}}
	want := prepareHashJobsAff5844(&reference)
	got := updated.prepareHashJobs()
	if !slices.EqualFunc(got, want, func(a, b candidate) bool { return a.path == b.path && a.info.Size() == b.info.Size() }) {
		t.Fatalf("budget=%d planning differs: got %#v want %#v", budget, got, want)
	}
	if !reflect.DeepEqual(updated.result, reference.result) {
		t.Fatalf("budget=%d result differs: got %#v want %#v", budget, updated.result, reference.result)
	}
	if cap(got) > 0 {
		for _, file := range got[len(got):cap(got)] {
			if file.info != nil || file.path != "" {
				t.Fatal("discarded candidate references retained")
			}
		}
	}
}

// Pure planning excludes filesystem/cache and platform identity effects. Both
// paths copy the same shuffled discovery input into reusable storage per scan.
func BenchmarkHashPlanning(b *testing.B) {
	for _, distribution := range []string{"unique", "pairs", "same"} {
		for _, count := range []int{1000, 10000} {
			b.Run(fmt.Sprintf("%s_%d", distribution, count), func(b *testing.B) {
				files := make([]candidate, count)
				random := rand.New(rand.NewPCG(7, 13))
				for i := range files {
					size := i
					if distribution == "pairs" {
						size = i / 2
					}
					if distribution == "same" {
						size = 32
					}
					files[i] = candidate{path: fmt.Sprintf("file-%06d", i), info: planningFileInfo{int64(size)}}
				}
				random.Shuffle(len(files), func(i, j int) { files[i], files[j] = files[j], files[i] })
				for _, baseline := range []bool{true, false} {
					name := "candidate"
					if baseline {
						name = "aff5844"
					}
					b.Run(name, func(b *testing.B) {
						storage := make([]candidate, len(files))
						b.ReportAllocs()
						for b.Loop() {
							copy(storage, files)
							s := scanner{files: storage}
							var jobs []candidate
							if baseline {
								jobs = prepareHashJobsAff5844(&s)
							} else {
								jobs = s.prepareHashJobs()
							}
							want := count
							if distribution == "unique" {
								want = 0
							}
							if len(jobs) != want {
								b.Fatalf("jobs=%d want %d", len(jobs), want)
							}
						}
					})
				}
			})
		}
	}
}

// The old worker's exact-size growth is retained to measure buffer churn with
// the same hashFile implementation and identical on-disk input.
func hashWorkerAff5844(ctx context.Context, jobs []candidate, outcomes []hashOutcome, next *atomic.Int64, wait *sync.WaitGroup) {
	defer wait.Done()
	var buffer []byte
	for {
		index := int(next.Add(1) - 1)
		if index >= len(jobs) {
			return
		}
		needed := int(min(jobs[index].info.Size(), int64(hashBufferSize)))
		if len(buffer) < needed {
			buffer = make([]byte, needed)
		}
		outcomes[index] = hashFile(ctx, jobs[index], buffer)
	}
}

func BenchmarkHashWorkerGrowth(b *testing.B) {
	root := b.TempDir()
	jobs := make([]candidate, 1024)
	digests := make([]string, len(jobs))
	var totalBytes int64
	for i := range jobs {
		size := (i + 1) * 16
		content := bytes.Repeat([]byte{byte(i)}, size)
		path := filepath.Join(root, fmt.Sprintf("file-%04d", i))
		if err := os.WriteFile(path, content, 0o644); err != nil {
			b.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			b.Fatal(err)
		}
		jobs[i] = candidate{path: path, info: info}
		digest := sha256.Sum256(content)
		digests[i] = hex.EncodeToString(digest[:])
		totalBytes += int64(size)
	}
	// Warm both the OS cache and lazy file identity metadata before comparing
	// workers. Cold filesystem latency is not the buffer-allocation workload.
	warmBuffer := make([]byte, hashBufferSize)
	for i, job := range jobs {
		outcome := hashFile(context.Background(), job, warmBuffer)
		if !outcome.done || outcome.digest != digests[i] {
			b.Fatalf("warmup failed at %d: %#v", i, outcome)
		}
	}
	for _, baseline := range []bool{true, false} {
		name := "candidate"
		if baseline {
			name = "aff5844"
		}
		b.Run(name, func(b *testing.B) {
			outcomes := make([]hashOutcome, len(jobs))
			b.ReportAllocs()
			b.SetBytes(totalBytes)
			for b.Loop() {
				var next atomic.Int64
				var wait sync.WaitGroup
				wait.Add(1)
				if baseline {
					hashWorkerAff5844(context.Background(), jobs, outcomes, &next, &wait)
				} else {
					hashWorker(context.Background(), jobs, outcomes, &next, &wait)
				}
				wait.Wait()
				for i, outcome := range outcomes {
					if !outcome.done || outcome.bytes != jobs[i].info.Size() || outcome.digest != digests[i] {
						b.Fatalf("incorrect hash at %d: %#v", i, outcome)
					}
				}
			}
		})
	}
}

func TestHashWorkerCancellationBeforeClaimingJob(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	jobs := []candidate{{path: "must-not-be-opened", info: planningFileInfo{math.MaxInt64}}}
	outcomes := make([]hashOutcome, len(jobs))
	var next atomic.Int64
	var wait sync.WaitGroup
	wait.Add(1)
	hashWorker(ctx, jobs, outcomes, &next, &wait)
	wait.Wait()
	if next.Load() != 0 || outcomes[0].done || outcomes[0].warn != nil {
		t.Fatalf("canceled worker consumed a job: next=%d outcomes=%#v", next.Load(), outcomes)
	}
}

func TestHashWorkerIncreasingSizesPreserveDigests(t *testing.T) {
	root := t.TempDir()
	jobs := make([]candidate, 65)
	expected := make([]string, len(jobs))
	for i := range jobs {
		size := i + 1
		if i == len(jobs)-1 {
			size = hashBufferSize + 1
		}
		content := bytes.Repeat([]byte{byte(i)}, size)
		path := filepath.Join(root, fmt.Sprintf("file-%04d", i))
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		jobs[i] = candidate{path: path, info: info}
		digest := sha256.Sum256(content)
		expected[i] = hex.EncodeToString(digest[:])
	}
	for _, workers := range []int{1, 4} {
		s := scanner{ctx: context.Background(), options: Options{Workers: workers}}
		outcomes := s.hashJobs(jobs)
		for i, outcome := range outcomes {
			if !outcome.done || outcome.warn != nil || outcome.bytes != jobs[i].info.Size() || outcome.digest != expected[i] {
				t.Fatalf("workers=%d job=%d outcome=%#v", workers, i, outcome)
			}
		}
	}
}
