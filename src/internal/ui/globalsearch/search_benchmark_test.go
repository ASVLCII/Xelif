package globalsearch

// This opt-in experiment deliberately calls each provider directly. It cannot
// accidentally benchmark the application's fallback chain.
import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const benchSearchLimit = 20000

type benchEntry struct {
	Path      string
	Directory bool
}
type benchQuery struct{ Name, Literal string }
type benchManifest struct {
	Seed        int
	Nonce, Root string
	Entries     []benchEntry
	Queries     []benchQuery
}
type benchSample struct {
	Provider, Query, Phase string
	Iteration              int
	Nanoseconds            int64
	Count                  int
	Valid                  bool
	Error                  string `json:",omitempty"`
}
type benchSummary struct {
	Provider, Query string
	MedianNS, P95NS int64
	Samples         int
}
type benchReport struct {
	StartedUTC, OS, Arch, CPU, GoVersion, GoExecutable, Executable, ESExecutable, Root string
	Seed, ResultLimit, VisitLimit                                                      int
	ReadinessBudget                                                                    string
	Status                                                                             map[string]string
	Correctness                                                                        map[string]string
	Samples                                                                            []benchSample
	Summaries                                                                          []benchSummary
	ComparisonValid                                                                    bool
}
type benchProvider struct {
	name   string
	search func(context.Context, string) Response
}

// Independent oracle: only the manifest's literal basenames and directory bits
// are used. No production matching/parser functions contribute expected values.
func benchExpected(m benchManifest, literal string) map[string]bool {
	want := map[string]bool{}
	for _, e := range m.Entries {
		if strings.Contains(strings.ToLower(filepath.Base(e.Path)), strings.ToLower(literal)) {
			want[filepath.Clean(e.Path)] = e.Directory
		}
	}
	return want
}

func benchValidate(root string, want map[string]bool, response Response, resultCap int) error {
	if response.Err != nil {
		return fmt.Errorf("provider error: %w", response.Err)
	}
	if response.Partial {
		return errors.New("partial response")
	}
	if resultCap <= len(want) || resultCap > 50000 || len(response.Results) >= resultCap {
		return errors.New("unsafe or reached result cap")
	}
	seen := map[string]bool{}
	for _, result := range response.Results {
		rel, err := filepath.Rel(root, result.Path)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("result outside corpus: %q", result.Path)
		}
		rel = filepath.Clean(rel)
		if seen[rel] {
			return fmt.Errorf("duplicate: %q", rel)
		}
		seen[rel] = true
		directory, exists := want[rel]
		if !exists {
			return fmt.Errorf("extra: %q", rel)
		}
		if directory != result.Directory {
			return fmt.Errorf("wrong directory flag: %q", rel)
		}
	}
	if len(seen) != len(want) {
		return fmt.Errorf("missing results: got %d want %d", len(seen), len(want))
	}
	return nil
}

func TestSearchBenchmarkValidator(t *testing.T) {
	root := t.TempDir()
	want := map[string]bool{"a.txt": false, "folder": true}
	good := []Result{{Path: filepath.Join(root, "a.txt")}, {Path: filepath.Join(root, "folder"), Directory: true}}
	if err := benchValidate(root, want, Response{Results: good}, 10); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		response Response
		cap      int
	}{
		{"missing", Response{Results: good[:1]}, 10},
		{"duplicate", Response{Results: append(append([]Result{}, good...), good[0])}, 10},
		{
			"extra",
			Response{Results: append(append([]Result{}, good...), Result{Path: filepath.Join(root, "extra")})},
			10,
		},
		{"wrong flag", Response{Results: []Result{good[0], {Path: good[1].Path}}}, 10},
		{"partial", Response{Results: good, Partial: true}, 10},
		{"canceled", Response{Err: context.Canceled}, 10},
		{"error", Response{Err: errors.New("failure")}, 10},
		{"cap", Response{Results: good}, 2},
		{"too high cap", Response{Results: good}, 50001},
		{"outside", Response{Results: []Result{{Path: filepath.Join(root, "..", "outside")}}}, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if benchValidate(root, want, tc.response, tc.cap) == nil {
				t.Fatal("accepted invalid response")
			}
		})
	}
}

func benchWriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

//nolint:gocognit // One opt-in fixture lifecycle with explicit recovery and integrity checks.
func benchCreateCorpus(
	parent string,
	logf func(string, ...any),
) (benchManifest, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return benchManifest{}, err
	}
	nonce := "xb" + hex.EncodeToString(random[:])
	if err := os.MkdirAll(parent, 0700); err != nil {
		return benchManifest{}, err
	}
	root := filepath.Join(parent, nonce)
	if err := os.Mkdir(root, 0700); err != nil {
		return benchManifest{}, err
	}
	m := benchManifest{Seed: 20261018, Nonce: nonce, Root: root}
	// The fixed integer mixer is reproducible independently of Go RNG changes.
	for d := range 100 {
		dir := fmt.Sprintf("%s-folder-%03d", nonce, d)
		m.Entries = append(m.Entries, benchEntry{dir, true})
		for f := range 100 {
			i := d*100 + f
			category := "many"
			switch {
			case i < 3:
				category = "few"
			case i == 3:
				category = "MiXeD"
			case i == 4:
				category = "punct.[x]+(a)"
			case i == 5:
				category = "日本語-é"
			}
			name := fmt.Sprintf("%s-%s-%08x-%05d.txt", nonce, category, uint32(i)*1664525+20261018, i)
			rel := filepath.Join(dir, name)
			m.Entries = append(m.Entries, benchEntry{rel, false})
		}
	}
	m.Queries = []benchQuery{
		{"many", nonce + "-many"},
		{"few", nonce + "-few"},
		{"zero", nonce + "-absent"},
		{"ascii-case", strings.ToUpper(nonce + "-mixed")},
		{"punctuation", nonce + "-punct.[x]+(a)"},
		{"folders", nonce + "-folder"},
		{"unicode", nonce + "-日本語-é"},
	}
	// Persist the complete plan before materializing it. A retained manifest
	// alone is not proof of completion; setup.json must say complete.
	if err := benchWriteJSON(filepath.Join(root, "manifest.json"), m); err != nil {
		return m, err
	}
	if err := benchWriteJSON(
		filepath.Join(root, "setup.json"),
		map[string]any{"Status": "creating", "PlannedFiles": 10000, "PlannedDirectories": 100, "Workers": 8},
	); err != nil {
		return m, err
	}
	logf("setup: root=%s planned=10000 empty files, 100 directories, workers=8", root)
	for _, entry := range m.Entries {
		if entry.Directory {
			if err := os.Mkdir(filepath.Join(root, entry.Path), 0700); err != nil {
				return m, err
			}
		}
	}
	logf("setup: 100 directories created; starting empty-file creators")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobs := make(chan string, 8)
	var workers sync.WaitGroup
	var completed atomic.Int64
	var firstError error
	var errorOnce sync.Once
	fail := func(err error) { errorOnce.Do(func() { firstError = err; cancel() }) }
	for range 8 {
		workers.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case path, ok := <-jobs:
					if !ok {
						return
					}
					file, err := os.OpenFile(filepath.Join(root, path), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
					if err != nil {
						fail(fmt.Errorf("create %s: %w", path, err))
						return
					}
					if err = file.Close(); err != nil {
						fail(fmt.Errorf("close %s: %w", path, err))
						return
					}
					if count := completed.Add(1); count%1000 == 0 {
						logf("setup: %d/10000 empty files created", count)
					}
				}
			}
		})
	}
dispatch:
	for _, entry := range m.Entries {
		if entry.Directory {
			continue
		}
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- entry.Path:
		}
	}
	close(jobs)
	workers.Wait()
	if firstError != nil {
		return m, firstError
	}
	if completed.Load() != 10000 {
		return m, fmt.Errorf("setup incomplete: %d/10000 files", completed.Load())
	}
	if err := benchWriteJSON(
		filepath.Join(root, "setup.json"),
		map[string]any{"Status": "complete", "CreatedFiles": completed.Load(), "CreatedDirectories": 100, "Workers": 8},
	); err != nil {
		return m, err
	}
	logf("setup: complete; beginning provider readiness (outside timing)")
	return m, nil
}

//nolint:gocognit,gocyclo,cyclop,maintidx // End-to-end benchmark keeps readiness, parity, and timing gates together.
func TestSearchProviderBenchmark(
	t *testing.T,
) {
	if os.Getenv("XELIF_BENCH_SEARCH") != "1" {
		t.Skip("opt in with XELIF_BENCH_SEARCH=1; see docs/benchmarks/search.md")
	}
	parent := os.Getenv("XELIF_BENCH_ROOT")
	if parent == "" {
		parent = `X:\Xelif-work\benchmarks`
	}
	m, err := benchCreateCorpus(parent, t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	executable, _ := os.Executable()
	esPath, _ := exec.LookPath("es.exe")
	goPath, _ := exec.LookPath("go")
	report := benchReport{
		StartedUTC:      time.Now().UTC().Format(time.RFC3339),
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
		CPU:             os.Getenv("PROCESSOR_IDENTIFIER"),
		GoVersion:       runtime.Version(),
		GoExecutable:    goPath,
		Executable:      executable,
		ESExecutable:    esPath,
		Root:            m.Root,
		Seed:            m.Seed,
		ResultLimit:     benchSearchLimit,
		VisitLimit:      benchSearchLimit,
		ReadinessBudget: "30s total across providers; two complete passes",
		Status:          map[string]string{},
		Correctness:     map[string]string{},
	}
	if runtime.GOOS == "windows" {
		report.GoExecutable += ".exe"
	}
	defer func() {
		if err := benchWriteJSON(filepath.Join(m.Root, "results.json"), report); err != nil {
			t.Error(err)
		}
		t.Logf("corpus and JSON retained: %s", m.Root)
	}()
	providers := []benchProvider{{"local", func(ctx context.Context, q string) Response {
		return SearchLocalQuery(
			ctx,
			m.Root,
			Query{Terms: []string{q}},
			LocalSearchOptions{ResultLimit: benchSearchLimit, VisitLimit: benchSearchLimit},
		)
	}},
		{"everything-ipc", func(ctx context.Context, q string) Response {
			r, e := searchEverythingDirect(ctx, q, benchSearchLimit)
			return Response{Results: r, Err: e}
		}}}
	if esPath != "" {
		providers = append(providers, benchProvider{"everything-es", func(ctx context.Context, q string) Response {
			r, e := searchEverything(
				ctx,
				esPath,
				q,
				benchSearchLimit,
				func(ctx context.Context, path string, args ...string) ([]byte, error) {
					return exec.CommandContext(ctx, path, args...).Output()
				},
			)
			return Response{Results: r, Err: e}
		}})
	} else {
		report.Status["everything-es"] = "unavailable: es.exe absent from PATH"
	}
	// First provider invocation is recorded before readiness and excluded from
	// distributions. It is not a cold-cache claim (no cache flushing occurs).
	invoke := func(parent context.Context, p benchProvider, q benchQuery, phase string, iteration int) error {
		ctx, cancel := context.WithTimeout(parent, 5*time.Second)
		defer cancel()
		start := time.Now()
		response := p.search(ctx, q.Literal)
		elapsed := time.Since(start)
		err := benchValidate(m.Root, benchExpected(m, q.Literal), response, benchSearchLimit)
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		sample := benchSample{
			Provider:    p.name,
			Query:       q.Name,
			Phase:       phase,
			Iteration:   iteration,
			Nanoseconds: elapsed.Nanoseconds(),
			Count:       len(response.Results),
			Valid:       err == nil,
		}
		if err != nil {
			sample.Error = err.Error()
		}
		report.Samples = append(report.Samples, sample)
		return err
	}
	readyCtx, stopReadiness := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopReadiness()
	active := []benchProvider{}
	for _, p := range providers {
		t.Logf("readiness: provider=%s", p.name)
		firstErr := invoke(readyCtx, p, m.Queries[0], "first-invocation", 0)
		if firstErr != nil &&
			(strings.Contains(firstErr.Error(), "not running") || runtime.GOOS != "windows" && p.name != "local") {
			report.Status[p.name] = "unavailable: " + firstErr.Error()
			continue
		}
		passes := 0
		var lastErr error
		for passes < 2 && readyCtx.Err() == nil {
			lastErr = nil
			for _, q := range m.Queries {
				r := p.search(readyCtx, q.Literal)
				if e := benchValidate(m.Root, benchExpected(m, q.Literal), r, benchSearchLimit); e != nil {
					lastErr = e
					break
				}
			}
			if lastErr == nil {
				passes++
			} else {
				passes = 0
				select {
				case <-readyCtx.Done():
				case <-time.After(100 * time.Millisecond):
				}
			}
		}
		if passes != 2 {
			if lastErr == nil {
				lastErr = readyCtx.Err()
			}
			if p.name == "local" {
				report.Status[p.name] = fmt.Sprintf("invalid: local readiness failed: %v", lastErr)
			} else {
				report.Status[p.name] = fmt.Sprintf(
					"excluded: incomplete index or failed readiness before shared deadline: %v; local-only comparison unavailable",
					lastErr,
				)
			}
			continue
		}
		report.Status[p.name] = "ready: two exact complete passes"
		// Cancellation and Unicode are correctness probes, never timing claims.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r := p.search(ctx, m.Queries[0].Literal)
		if !errors.Is(r.Err, context.Canceled) || len(r.Results) != 0 {
			report.Status[p.name] = "invalid: pre-canceled request was not rejected"
			continue
		}
		report.Correctness[p.name] = "unicode exact set passed; pre-canceled request rejected"
		active = append(active, p)
	}
	for _, q := range m.Queries[:6] {
		t.Logf("timing: query=%s providers=%d; 2 warmups + 30 samples each", q.Name, len(active))
		for warm := range 2 {
			for _, p := range active {
				if e := invoke(context.Background(), p, q, "warmup", warm); e != nil {
					t.Errorf("%s %s warmup: %v", p.name, q.Name, e)
					return
				}
			}
		}
		// Rotate order by sample: each provider receives 30 observations per query.
		for i := range 30 {
			for j := range active {
				p := active[(i+j)%len(active)]
				if e := invoke(context.Background(), p, q, "measured", i); e != nil {
					t.Errorf("%s %s sample: %v", p.name, q.Name, e)
					return
				}
			}
		}
		for _, p := range active {
			values := []int64{}
			for _, s := range report.Samples {
				if s.Phase == "measured" && s.Provider == p.name && s.Query == q.Name {
					values = append(values, s.Nanoseconds)
				}
			}
			sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
			if len(values) == 30 {
				report.Summaries = append(
					report.Summaries,
					benchSummary{
						p.name,
						q.Name,
						(values[14] + values[15]) / 2,
						values[int(math.Ceil(.95*float64(len(values))))-1],
						len(values),
					},
				)
			}
		}
	}
	report.ComparisonValid = len(active) > 1
	for _, status := range report.Status {
		if strings.HasPrefix(status, "excluded:") {
			report.ComparisonValid = false
		}
		if strings.HasPrefix(status, "invalid:") {
			report.ComparisonValid = false
			t.Error(status)
		}
	}
	if len(active) == 0 {
		t.Error("no valid provider")
	}
}
