//go:build freebsd

package vigil

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"unsafe"
)

const (
	ctlKern           = 1
	kernProc          = 14
	kernProcAll       = 0
	kernProcPID       = 1
	kernProcArgs      = 7
	kinfoStructSize   = 1088 // sizeof(struct kinfo_proc) on FreeBSD amd64
)

// Field offsets within struct kinfo_proc on FreeBSD.
const (
	offPid  = 72  // ki_pid (int32)
	offPpid = 76  // ki_ppid (int32)
	offUid  = 88  // ki_uid (uint32)
	offComm = 447 // ki_comm (COMMLEN+1 = 20 bytes)
)

func list(ctx context.Context) ([]Process, error) {
	buf, err := sysctlProc(kernProcAll, 0)
	if err != nil {
		return nil, fmt.Errorf("vigil: sysctl: %w", err)
	}
	n := len(buf) / kinfoStructSize
	procs := make([]Process, 0, n)
	for i := 0; i < n; i++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		p := parseKinfo(buf[i*kinfoStructSize : (i+1)*kinfoStructSize])
		fillFreeBSDDetails(&p)
		procs = append(procs, p)
	}
	return procs, nil
}

func find(ctx context.Context, pid int) (*Process, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	buf, err := sysctlProc(kernProcPID, pid)
	if err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil, nil
		}
		return nil, err
	}
	if len(buf) < kinfoStructSize {
		return nil, nil
	}
	p := parseKinfo(buf[:kinfoStructSize])
	fillFreeBSDDetails(&p)
	return &p, nil
}

func sysctlProc(op int, arg int) ([]byte, error) {
	mib := [4]int32{ctlKern, kernProc, int32(op), int32(arg)}
	var size uintptr
	_, _, errno := syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		4,
		0,
		uintptr(unsafe.Pointer(&size)),
		0,
		0,
	)
	if errno != 0 {
		return nil, errno
	}
	if size == 0 {
		return nil, nil
	}
	buf := make([]byte, size)
	_, _, errno = syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		4,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
		0,
	)
	if errno != 0 {
		return nil, errno
	}
	return buf[:size], nil
}

func parseKinfo(data []byte) Process {
	p := Process{
		PID:  int(int32(binary.LittleEndian.Uint32(data[offPid : offPid+4]))),
		PPID: int(int32(binary.LittleEndian.Uint32(data[offPpid : offPpid+4]))),
	}
	uid := binary.LittleEndian.Uint32(data[offUid : offUid+4])
	p.Executable = cstring(data[offComm : offComm+20])
	p.User = lookupUID(uid)
	return p
}

func fillFreeBSDDetails(p *Process) {
	// Full path via /proc/PID/file (procfs)
	link, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(p.PID), "file"))
	if err == nil {
		p.Path = link
	}

	// Args via sysctl KERN_PROC_ARGS
	args, err := procArgs(p.PID)
	if err == nil && len(args) > 0 {
		p.Args = args
		base := filepath.Base(args[0])
		if len(base) > len(p.Executable) {
			p.Executable = base
		}
	}
}

func procArgs(pid int) ([]string, error) {
	mib := [4]int32{ctlKern, kernProc, kernProcArgs, int32(pid)}
	var size uintptr
	_, _, errno := syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		4,
		0,
		uintptr(unsafe.Pointer(&size)),
		0,
		0,
	)
	if errno != 0 {
		return nil, errno
	}
	if size == 0 {
		return nil, nil
	}
	buf := make([]byte, size)
	_, _, errno = syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		4,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
		0,
	)
	if errno != 0 {
		return nil, errno
	}
	buf = buf[:size]
	// NUL-delimited args
	buf = bytes.TrimRight(buf, "\x00")
	if len(buf) == 0 {
		return nil, nil
	}
	parts := bytes.Split(buf, []byte{0})
	args := make([]string, len(parts))
	for i, part := range parts {
		args[i] = string(part)
	}
	return args, nil
}

func cstring(b []byte) string {
	idx := bytes.IndexByte(b, 0)
	if idx < 0 {
		return string(b)
	}
	return string(b[:idx])
}

func lookupUID(uid uint32) string {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}
