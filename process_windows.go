//go:build windows

package vigil

import (
	"context"
	"fmt"
	"syscall"
	"unsafe"
)

var (
	modkernel32 = syscall.NewLazyDLL("kernel32.dll")
	modadvapi32 = syscall.NewLazyDLL("advapi32.dll")

	procCreateToolhelp32Snapshot = modkernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First           = modkernel32.NewProc("Process32FirstW")
	procProcess32Next            = modkernel32.NewProc("Process32NextW")
	procOpenProcess              = modkernel32.NewProc("OpenProcess")
	procQueryFullProcessImageName = modkernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle              = modkernel32.NewProc("CloseHandle")
	procOpenProcessToken         = modadvapi32.NewProc("OpenProcessToken")
	procGetTokenInformation      = modadvapi32.NewProc("GetTokenInformation")
	procLookupAccountSid         = modadvapi32.NewProc("LookupAccountSidW")
)

const (
	thSnapProcess     = 0x00000002
	invalidHandleVal  = ^uintptr(0)
	processQueryInfo  = 0x0400
	processVMRead     = 0x0010
	tokenQuery        = 0x0008
	tokenUser         = 1
	maxPath           = 260
)

// processEntry32W mirrors PROCESSENTRY32W.
type processEntry32W struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	CntThreads      uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [maxPath]uint16
}

func list(ctx context.Context) ([]Process, error) {
	snap, _, err := procCreateToolhelp32Snapshot.Call(thSnapProcess, 0)
	if snap == invalidHandleVal {
		return nil, fmt.Errorf("vigil: CreateToolhelp32Snapshot: %w", err)
	}
	defer procCloseHandle.Call(snap) //nolint:errcheck

	var entry processEntry32W
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, err := procProcess32First.Call(snap, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return nil, fmt.Errorf("vigil: Process32First: %w", err)
	}

	var procs []Process
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		p := snapshotToProcess(&entry)
		fillWindowsDetails(&p)
		procs = append(procs, p)

		entry.Size = uint32(unsafe.Sizeof(entry))
		ret, _, _ = procProcess32Next.Call(snap, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}
	return procs, nil
}

func find(ctx context.Context, pid int) (*Process, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	procs, err := list(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range procs {
		if p.PID == pid {
			return &p, nil
		}
	}
	return nil, nil
}

func snapshotToProcess(entry *processEntry32W) Process {
	return Process{
		PID:        int(entry.ProcessID),
		PPID:       int(entry.ParentProcessID),
		Executable: utf16ToString(entry.ExeFile[:]),
	}
}

func fillWindowsDetails(p *Process) {
	handle, _, _ := procOpenProcess.Call(processQueryInfo|processVMRead, 0, uintptr(p.PID))
	if handle == 0 {
		return
	}
	defer procCloseHandle.Call(handle) //nolint:errcheck

	// Get full path
	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	ret, _, _ := procQueryFullProcessImageName.Call(handle, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ret != 0 {
		p.Path = utf16ToString(buf[:size])
	}

	// Get process owner
	p.User = getProcessUser(handle)
}

func getProcessUser(processHandle uintptr) string {
	var tokenHandle syscall.Token
	ret, _, _ := procOpenProcessToken.Call(processHandle, tokenQuery, uintptr(unsafe.Pointer(&tokenHandle)))
	if ret == 0 {
		return ""
	}
	defer tokenHandle.Close() //nolint:errcheck

	// Get token user info
	var needed uint32
	procGetTokenInformation.Call(uintptr(tokenHandle), tokenUser, 0, 0, uintptr(unsafe.Pointer(&needed))) //nolint:errcheck
	if needed == 0 {
		return ""
	}

	buf := make([]byte, needed)
	ret, _, _ = procGetTokenInformation.Call(uintptr(tokenHandle), tokenUser, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)))
	if ret == 0 {
		return ""
	}

	// TOKEN_USER: first field is SID pointer
	sid := *(*uintptr)(unsafe.Pointer(&buf[0]))

	// LookupAccountSid
	nameLen := uint32(256)
	domainLen := uint32(256)
	name := make([]uint16, nameLen)
	domain := make([]uint16, domainLen)
	var sidUse uint32
	ret, _, _ = procLookupAccountSid.Call(
		0,
		sid,
		uintptr(unsafe.Pointer(&name[0])),
		uintptr(unsafe.Pointer(&nameLen)),
		uintptr(unsafe.Pointer(&domain[0])),
		uintptr(unsafe.Pointer(&domainLen)),
		uintptr(unsafe.Pointer(&sidUse)),
	)
	if ret == 0 {
		return ""
	}

	domainStr := utf16ToString(domain[:domainLen])
	nameStr := utf16ToString(name[:nameLen])
	if domainStr != "" {
		return domainStr + `\` + nameStr
	}
	return nameStr
}

func utf16ToString(s []uint16) string {
	for i, v := range s {
		if v == 0 {
			return syscall.UTF16ToString(s[:i])
		}
	}
	return syscall.UTF16ToString(s)
}
