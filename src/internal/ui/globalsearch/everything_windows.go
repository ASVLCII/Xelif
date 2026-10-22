package globalsearch

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Native Win32 calls are confined to this file. Only system user32.dll is
// loaded; no SDK DLL, command subprocess, or third-party code is loaded.
var everythingIPC = struct { //nolint:gochecknoglobals // One registered Win32 callback/class for the process lifetime.
	once     sync.Once
	err      error
	callback uintptr
	requests sync.Map
	nextID   atomic.Uint32
	dll      *windows.LazyDLL
}{dll: windows.NewLazySystemDLL("user32.dll")}

const (
	ipcCopyDataMessage = 0x004a
	ipcUnicodeQuery    = 18
	ipcReplyID         = 0
	ipcSendTimeout     = 200
	ipcPollInterval    = 10 * time.Millisecond
	ipcQueryTimeout    = time.Second
	ipcAbortIfHung     = 2
)

type ipcCopyData struct {
	id   uintptr
	size uint32
	data unsafe.Pointer
}

// The receiving header uses integers for native addresses. Copying an
// untrusted address into a Go pointer would make it visible to the GC.
// Its layout matches COPYDATASTRUCT, including platform-specific padding.
type ipcCopyDataHeader struct {
	id   uintptr
	size uint32
	data uintptr
}

type ipcWindowClass struct {
	style       uint32
	callback    uintptr
	classExtra  int32
	windowExtra int32
	instance    uintptr
	icon        uintptr
	cursor      uintptr
	background  uintptr
	menu        *uint16
	name        *uint16
}

type ipcMessage struct {
	window  uintptr
	message uint32
	wparam  uintptr
	lparam  uintptr
	time    uint32
	x       int32
	y       int32
	private uint32
}

type ipcReply struct {
	data []byte
	err  error
}

type ipcPendingRequest struct {
	server  uintptr
	replyID uintptr
	replies chan ipcReply
}

func ipcProc(name string) *windows.LazyProc { return everythingIPC.dll.NewProc(name) }

// The callback copies WM_COPYDATA memory before returning, as required by
// Win32. Parsing happens outside the callback and validates counts/offsets.
func everythingWindowProc(hwnd, message, wparam, lparam uintptr) uintptr {
	if message == ipcCopyDataMessage && lparam != 0 {
		if value, ok := everythingIPC.requests.Load(hwnd); ok {
			request, valid := value.(ipcPendingRequest)
			if !valid {
				return 0
			}
			if wparam != request.server {
				return 0
			}
			if result, handled := copyEverythingReply(lparam, request.replyID); handled {
				select {
				case request.replies <- result:
				default:
				}
				return 1
			}
		}
	}
	result, _, _ := ipcProc("DefWindowProcW").Call(hwnd, message, wparam, lparam)
	return result
}

// ReadProcessMemory validates native addresses and copies into Go-owned
// memory without converting an arbitrary LPARAM into a Go pointer. Both
// copies finish while WM_COPYDATA's borrowed memory is still valid.
func copyEverythingReply(address, replyID uintptr) (ipcReply, bool) {
	var header ipcCopyDataHeader
	if err := readEverythingMemory(address, (*byte)(unsafe.Pointer(&header)), unsafe.Sizeof(header)); err != nil {
		return ipcReply{err: fmt.Errorf("invalid Everything IPC header: %w", err)}, true
	}
	if header.id != replyID {
		return ipcReply{}, false
	}
	if header.data == 0 || header.size < ipcListHeaderBytes || header.size > ipcMaxReplyBytes {
		return ipcReply{err: errors.New("invalid Everything IPC payload")}, true
	}
	data := make([]byte, int(header.size))
	if err := readEverythingMemory(header.data, &data[0], uintptr(len(data))); err != nil {
		return ipcReply{err: fmt.Errorf("invalid Everything IPC payload: %w", err)}, true
	}
	return ipcReply{data: data}, true
}

func readEverythingMemory(address uintptr, buffer *byte, size uintptr) error {
	var read uintptr
	if err := windows.ReadProcessMemory(windows.CurrentProcess(), address, buffer, size, &read); err != nil {
		return err
	}
	if read != size {
		return errors.New("incomplete native memory read")
	}
	return nil
}

func registerEverythingWindow() {
	name, _ := windows.UTF16PtrFromString("XelifEverythingQuery")
	everythingIPC.callback = syscall.NewCallback(everythingWindowProc)
	instance, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	class := ipcWindowClass{callback: everythingIPC.callback, name: name, instance: instance}
	atom, _, err := ipcProc("RegisterClassW").Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		everythingIPC.err = fmt.Errorf("register Everything reply window: %w", err)
	}
}

func searchEverythingDirect(ctx context.Context, query string, limit int) ([]Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Bound the direct attempt separately so an unresponsive provider leaves
	// time for the local fallback within the modal's overall search deadline.
	directCtx, cancel := context.WithTimeout(ctx, ipcQueryTimeout)
	defer cancel()
	results, err := queryEverythingWindow(directCtx, query, limit)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return nil, errors.New("everything IPC timed out")
	}
	return results, err
}

func queryEverythingWindow(ctx context.Context, query string, limit int) ([]Result, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	className, _ := windows.UTF16PtrFromString("EVERYTHING_TASKBAR_NOTIFICATION")
	server, _, _ := ipcProc("FindWindowW").Call(uintptr(unsafe.Pointer(className)), 0)
	if server == 0 {
		return nil, errors.New("everything is not running")
	}
	everythingIPC.once.Do(registerEverythingWindow)
	if everythingIPC.err != nil {
		return nil, everythingIPC.err
	}
	name, _ := windows.UTF16PtrFromString("XelifEverythingQuery")
	title, _ := windows.UTF16PtrFromString("")
	instance, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	hwnd, _, err := ipcProc("CreateWindowExW").Call(
		0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(title)), 0, 0, 0, 0, 0, 0, 0, instance, 0,
	)
	if hwnd == 0 {
		return nil, fmt.Errorf("create Everything reply window: %w", err)
	}
	defer func() { _, _, _ = ipcProc("DestroyWindow").Call(hwnd) }()
	// Match the SDK's per-window filter: Everything may run at a different
	// integrity level. Only this hidden reply window accepts WM_COPYDATA;
	// the callback also checks the expected server HWND and reply identifier.
	allowed, _, filterErr := ipcProc("ChangeWindowMessageFilterEx").Call(hwnd, ipcCopyDataMessage, 1, 0)
	if allowed == 0 {
		return nil, fmt.Errorf("allow Everything IPC replies: %w", filterErr)
	}
	replies := make(chan ipcReply, 1)
	// The SDK permits caller-chosen IDs. A unique ID prevents a late reply
	// from an older query being accepted if Windows reuses its HWND.
	replyID := everythingIPC.nextID.Add(1)
	everythingIPC.requests.Store(hwnd, ipcPendingRequest{server: server, replyID: uintptr(replyID), replies: replies})
	defer everythingIPC.requests.Delete(hwnd)
	//nolint:gosec // G115: QUERY2 uses a DWORD HWND, matching the SDK's Win32/Win64 ABI.
	packet := everythingQueryPacket(uint32(hwnd), replyID, query, limit)
	if len(packet) > ipcMaxReplyBytes {
		return nil, errors.New("everything query exceeds the safety limit")
	}
	//nolint:gosec // G115: packet length was bounded to 16 MiB immediately above.
	data := ipcCopyData{id: ipcUnicodeQuery, size: uint32(len(packet)), data: unsafe.Pointer(&packet[0])}
	var accepted uintptr
	sent, _, sendErr := ipcProc("SendMessageTimeoutW").Call(
		server, ipcCopyDataMessage, hwnd, uintptr(unsafe.Pointer(&data)),
		ipcAbortIfHung, ipcSendTimeout, uintptr(unsafe.Pointer(&accepted)),
	)
	runtime.KeepAlive(packet)
	if sent == 0 || accepted == 0 {
		return nil, fmt.Errorf("everything rejected IPC query: %w", sendErr)
	}
	return awaitEverythingReply(ctx, replies, limit)
}

func awaitEverythingReply(ctx context.Context, replies <-chan ipcReply, limit int) ([]Result, error) {
	ticker := time.NewTicker(ipcPollInterval)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var message ipcMessage
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			found, _, _ := ipcProc("PeekMessageW").Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, 1)
			if found == 0 {
				break
			}
			// DispatchMessage's return is a window-procedure result, not a Win32 error.
			_, _, _ = ipcProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&message)))
		}
		select {
		case reply := <-replies:
			if reply.err != nil {
				return nil, reply.err
			}
			return parseEverythingReply(reply.data, limit)
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
