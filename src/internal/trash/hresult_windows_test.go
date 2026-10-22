//go:build windows

package trash

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

func TestProgressSinkNativeCallbackABI(t *testing.T) {
	sink := newProgressSink()
	defer runtime.KeepAlive(sink)
	var receiver unsafe.Pointer
	hr, _, _ := syscall.SyscallN(progressSinkVtbl.queryInterface,
		uintptr(unsafe.Pointer(sink)), uintptr(unsafe.Pointer(&iidIUnknown)), uintptr(unsafe.Pointer(&receiver)))
	if failed(hr) || receiver != unsafe.Pointer(sink) || sink.refs != 2 {
		t.Fatalf("QueryInterface hr=%#x receiver=%p refs=%d", hr, receiver, sink.refs)
	}
	refs, _, _ := syscall.SyscallN(progressSinkVtbl.release, uintptr(unsafe.Pointer(sink)))
	if refs != 1 || sink.refs != 1 {
		t.Fatalf("Release refs=%d, stored=%d", refs, sink.refs)
	}
	unknown := guid{data1: 0xffffffff}
	hr, _, _ = syscall.SyscallN(progressSinkVtbl.queryInterface,
		uintptr(unsafe.Pointer(sink)), uintptr(unsafe.Pointer(&unknown)), uintptr(unsafe.Pointer(&receiver)))
	if hr&hresultMask != eNoInterface || receiver != nil || sink.refs != 1 {
		t.Fatalf("unknown IID hr=%#x receiver=%p refs=%d", hr, receiver, sink.refs)
	}
}

func TestHRESULTSeverityAndWidth(t *testing.T) {
	for _, tc := range []struct {
		value   uintptr
		failure bool
	}{
		{0, false}, {1, false}, {0x7fffffff, false},
		{0x80000000, true}, {0x80004005, true}, {0xffffffff, true},
	} {
		if got := failed(tc.value); got != tc.failure {
			t.Errorf("failed(%#x)=%v, want %v", tc.value, got, tc.failure)
		}
	}
	upper := ^uintptr(0) & ^uintptr(hresultMask)
	if failed(upper | 1) {
		t.Error("bits above HRESULT width must not turn S_FALSE into a failure")
	}
	if !failed(upper | 0x80004005) {
		t.Error("E_FAIL must retain its failure bit")
	}
	const want = "operation failed: HRESULT 0x80004005"
	if got := hresultError("operation", upper|0x80004005).Error(); got != want {
		t.Errorf("formatted HRESULT=%q, want %q", got, want)
	}
}
