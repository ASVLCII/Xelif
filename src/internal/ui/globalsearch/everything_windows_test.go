package globalsearch

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// Opt-in because ordinary CI should not depend on an installed, indexed
// Everything instance. This exercises the real IPC path, with no CLI fallback.
func TestEverythingNativeLive(t *testing.T) {
	if os.Getenv("XELIF_TEST_EVERYTHING") != "1" {
		t.Skip("set XELIF_TEST_EVERYTHING=1 with Everything running and Windows indexed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	results, err := searchEverythingDirect(ctx, "explorer.exe", 10)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	assert.LessOrEqual(t, len(results), 10)
	for _, result := range results {
		assert.Contains(t, strings.ToLower(result.Path), "explorer.exe")
		assert.False(t, result.Directory)
	}
}

func TestEverythingNativeReplyWindowCopiesPayload(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	everythingIPC.once.Do(registerEverythingWindow)
	require.NoError(t, everythingIPC.err)
	name, err := windows.UTF16PtrFromString("XelifEverythingQuery")
	require.NoError(t, err)
	hwnd, _, createErr := ipcProc("CreateWindowExW").Call(
		0, uintptr(unsafe.Pointer(name)), 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	)
	require.NotZero(t, hwnd, "%v", createErr)
	defer ipcProc("DestroyWindow").Call(hwnd)
	replies := make(chan ipcReply, 1)
	everythingIPC.requests.Store(hwnd, ipcPendingRequest{server: hwnd, replies: replies})
	defer everythingIPC.requests.Delete(hwnd)
	packet := testEverythingReply()
	expected := append([]byte(nil), packet...)
	data := ipcCopyData{id: ipcReplyID, size: uint32(len(packet)), data: unsafe.Pointer(&packet[0])}
	ipcProc("SendMessageW").Call(hwnd, ipcCopyDataMessage, hwnd, uintptr(unsafe.Pointer(&data)))
	select {
	case reply := <-replies:
		require.NoError(t, reply.err)
		packet[0] ^= 0xff
		assert.Equal(t, expected, reply.data, "reply must own its bytes after SendMessageW returns")
	default:
		t.Fatal("native reply callback did not deliver WM_COPYDATA")
	}
	// Keep sends on the window's owning OS thread. t.Run uses another
	// goroutine, whose synchronous SendMessageW would wait for this thread.
	ipcProc("SendMessageW").Call(hwnd, ipcCopyDataMessage, hwnd+1, uintptr(unsafe.Pointer(&data)))
	assert.Empty(t, replies, "unexpected senders must be ignored")
	data.id = ipcReplyID + 1
	ipcProc("SendMessageW").Call(hwnd, ipcCopyDataMessage, hwnd, uintptr(unsafe.Pointer(&data)))
	assert.Empty(t, replies, "unexpected reply identifiers must be ignored")
	data.id = ipcReplyID
	ipcProc("SendMessageW").Call(hwnd, ipcCopyDataMessage, hwnd, uintptr(unsafe.Pointer(&data)))
	ipcProc("SendMessageW").Call(hwnd, ipcCopyDataMessage, hwnd, uintptr(unsafe.Pointer(&data)))
	require.Len(t, replies, 1, "a full reply channel must not block the callback")
	reply := <-replies
	require.NoError(t, reply.err)
	assert.Equal(t, packet, reply.data)
	// A new request at the same HWND rejects the old request's reply ID.
	const nextReplyID = 42
	everythingIPC.requests.Store(hwnd, ipcPendingRequest{server: hwnd, replyID: nextReplyID, replies: replies})
	ipcProc("SendMessageW").Call(hwnd, ipcCopyDataMessage, hwnd, uintptr(unsafe.Pointer(&data)))
	require.Empty(t, replies, "late reply from the previous request must be ignored")
	data.id = nextReplyID
	ipcProc("SendMessageW").Call(hwnd, ipcCopyDataMessage, hwnd, uintptr(unsafe.Pointer(&data)))
	require.Len(t, replies, 1, "current request reply must still be accepted")
	require.NoError(t, (<-replies).err)
	runtime.KeepAlive(packet)
}

func TestEverythingCopyDataHeaderLayout(t *testing.T) {
	var sent ipcCopyData
	var received ipcCopyDataHeader
	assert.Equal(t, unsafe.Sizeof(sent), unsafe.Sizeof(received))
	assert.Equal(t, unsafe.Offsetof(sent.id), unsafe.Offsetof(received.id))
	assert.Equal(t, unsafe.Offsetof(sent.size), unsafe.Offsetof(received.size))
	assert.Equal(t, unsafe.Offsetof(sent.data), unsafe.Offsetof(received.data))
}

func TestEverythingCopyReplyValidation(t *testing.T) {
	packet := testEverythingReply()
	var pinned runtime.Pinner
	pinned.Pin(&packet[0])
	defer pinned.Unpin()
	validAddress := uintptr(unsafe.Pointer(&packet[0]))
	tests := []struct {
		name    string
		header  ipcCopyDataHeader
		handled bool
		invalid bool
	}{
		{"valid", ipcCopyDataHeader{ipcReplyID, uint32(len(packet)), validAddress}, true, false},
		{"nil payload", ipcCopyDataHeader{ipcReplyID, uint32(len(packet)), 0}, true, true},
		{"short payload", ipcCopyDataHeader{ipcReplyID, ipcListHeaderBytes - 1, validAddress}, true, true},
		{"oversized payload", ipcCopyDataHeader{ipcReplyID, ipcMaxReplyBytes + 1, validAddress}, true, true},
		{"unreadable payload", ipcCopyDataHeader{ipcReplyID, uint32(len(packet)), 1}, true, true},
		{"other reply ID ignores payload", ipcCopyDataHeader{ipcReplyID + 1, ipcMaxReplyBytes + 1, 1}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Native reads use integer addresses, so pin the Go-owned fixture
			// rather than depending on a stack address surviving a syscall.
			var headerPin runtime.Pinner
			headerPin.Pin(&tt.header)
			defer headerPin.Unpin()
			reply, handled := copyEverythingReply(uintptr(unsafe.Pointer(&tt.header)), ipcReplyID)
			require.Equal(t, tt.handled, handled)
			if tt.invalid {
				require.ErrorContains(t, reply.err, "invalid Everything IPC payload")
				assert.Nil(t, reply.data)
			} else {
				require.NoError(t, reply.err)
				if handled {
					assert.Equal(t, packet, reply.data)
				} else {
					assert.Nil(t, reply.data)
				}
			}
		})
	}
	for _, address := range []uintptr{0, 1} {
		reply, handled := copyEverythingReply(address, ipcReplyID)
		assert.True(t, handled)
		require.ErrorContains(t, reply.err, "invalid Everything IPC header")
		assert.Nil(t, reply.data)
	}
}

func TestEverythingCopyReplyRejectsPartialMemory(t *testing.T) {
	page := uintptr(os.Getpagesize())
	base, err := windows.VirtualAlloc(0, 2*page, windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, windows.VirtualFree(base, 0, windows.MEM_RELEASE)) })
	var oldProtection uint32
	require.NoError(t, windows.VirtualProtect(base+page, page, windows.PAGE_NOACCESS, &oldProtection))
	// Start in readable memory but cross into a page with no read access.
	boundary := base + page - 1
	reply, handled := copyEverythingReply(boundary, ipcReplyID)
	assert.True(t, handled)
	require.ErrorContains(t, reply.err, "invalid Everything IPC header")
	assert.Nil(t, reply.data)

	header := ipcCopyDataHeader{id: ipcReplyID, size: ipcListHeaderBytes, data: boundary}
	var pinned runtime.Pinner
	pinned.Pin(&header)
	defer pinned.Unpin()
	reply, handled = copyEverythingReply(uintptr(unsafe.Pointer(&header)), ipcReplyID)
	assert.True(t, handled)
	require.ErrorContains(t, reply.err, "invalid Everything IPC payload")
	assert.Nil(t, reply.data)
}

func TestEverythingNativeAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := searchEverythingDirect(ctx, "explorer.exe", 10)
	require.ErrorIs(t, err, context.Canceled)
}
