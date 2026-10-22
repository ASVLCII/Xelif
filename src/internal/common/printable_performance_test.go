package common

import (
	"crypto/sha256"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

func TestPrintableUnchangedNeedsNoAllocation(t *testing.T) {
	line := "\x1b[38;2;128;192;224m\uf07b tài-liệu 文件\x1b[0m\nname\u00a0tail"
	allocations := testing.AllocsPerRun(100, func() {
		if got := MakePrintableWithEscCheck(line, true); got != line {
			t.Fatal("permitted content changed")
		}
	})
	if allocations != 0 {
		t.Fatalf("unchanged line allocated %v times", allocations)
	}
}

// This same fixture is built against the pre-optimization source using a Go
// overlay. Compare its digest with the candidate, not just selected examples.
func TestPrintableOutputFingerprint(t *testing.T) {
	atoms := []string{
		"", "plain", "\x1b[31mred\x1b[0m", "\n", "\t", "\x00", "\x7f", "\x1f",
		"\ufffd", "\xff\xfe", "\xc0\xaf", "tài-liệu", "文件", "\uf07b", "\u00a0",
		"\u0085", "\u1680", "\u2000", "\u2003", "\u2028", "\u2029", "\u202f", "\u3000",
	}
	digest := sha256.New()
	for _, allowEsc := range []bool{false, true} {
		for _, first := range atoms {
			for _, second := range atoms {
				for _, third := range atoms {
					input := "prefix " + first + "X" + second + "tail" + third
					output := MakePrintableWithEscCheck(input, allowEsc)
					fmt.Fprintf(digest, "%t:%d:%s", allowEsc, len(output), output)
				}
			}
		}
	}
	t.Logf("SHA256 %x", digest.Sum(nil))
}

func BenchmarkMakePrintable(b *testing.B) {
	inputs := []struct{ name, text string }{
		{"styled", "\x1b[38;2;128;192;224m\uf07b tài-liệu 文件\x1b[0m"},
		{"trailing-control", strings.Repeat("abc 文件 ", 32) + "\x00"},
		{"tabs", "first\tsecond\n\x1b[31m文件\x1b[0m\tend"},
	}
	for _, input := range inputs {
		b.Run(input.name, func(b *testing.B) {
			b.ReportAllocs()
			var result string
			for b.Loop() {
				result = MakePrintableWithEscCheck(input.text, true)
			}
			runtime.KeepAlive(result)
		})
	}
}
