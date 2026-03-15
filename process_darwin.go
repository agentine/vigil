//go:build darwin

package vigil

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os/user"
	"strconv"
	"syscall"
	"unsafe"
)

// sysctl MIB constants
const (
	ctlKern        = 1
	kernProc       = 14
	kernProcAll    = 0
	kernProcPID    = 1
	kernProcArgs2  = 49 // KERN_PROCARGS2
)

func list(ctx context.Context) ([]Process, error) {
	kprocs, err := sysctl_kinfo_all()
	if err != nil {
		return nil, fmt.Errorf("vigil: sysctl: %w", err)
	}
	procs := make([]Process, 0, len(kprocs))
	for _, kp := range kprocs {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		p := kinfoToProcess(&kp)
		fillPath(&p)
		fillArgs(&p)
		procs = append(procs, p)
	}
	return procs, nil
}

func find(ctx context.Context, pid int) (*Process, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	kp, err := sysctl_kinfo_pid(pid)
	if err != nil {
		// Process not found
		return nil, nil
	}
	p := kinfoToProcess(kp)
	fillPath(&p)
	fillArgs(&p)
	return &p, nil
}

func kinfoToProcess(kp *kinfoProc) Process {
	p := Process{
		PID:  int(kp.Pid),
		PPID: int(kp.Ppid),
	}
	// Extract executable name from p_comm (null-terminated)
	p.Executable = cstring(kp.Comm[:])
	// Resolve UID to username
	p.User = lookupUID(kp.Uid)
	return p
}

func fillPath(p *Process) {
	path, err := procPidpath(p.PID)
	if err == nil && path != "" {
		p.Path = path
	}
}

func fillArgs(p *Process) {
	args, err := procArgs(p.PID)
	if err == nil && len(args) > 0 {
		p.Args = args
		// Use full executable name from args if comm was truncated
		if len(args[0]) > 0 {
			base := basename(args[0])
			if len(base) > len(p.Executable) {
				p.Executable = base
			}
		}
	}
}

// kinfoProc holds the fields we care about from struct kinfo_proc.
type kinfoProc struct {
	Pid  int32
	Ppid int32
	Uid  uint32
	Comm [16]byte
}

// sysctl_kinfo_all returns kinfo_proc for all processes.
func sysctl_kinfo_all() ([]kinfoProc, error) {
	mib := [4]int32{ctlKern, kernProc, kernProcAll, 0}
	return sysctl_kinfos(mib[:])
}

// sysctl_kinfo_pid returns kinfo_proc for a specific PID.
func sysctl_kinfo_pid(pid int) (*kinfoProc, error) {
	mib := [4]int32{ctlKern, kernProc, kernProcPID, int32(pid)}
	procs, err := sysctl_kinfos(mib[:])
	if err != nil {
		return nil, err
	}
	if len(procs) == 0 {
		return nil, fmt.Errorf("process %d not found", pid)
	}
	return &procs[0], nil
}

// sysctl_kinfos calls sysctl and parses the raw kinfo_proc structures.
func sysctl_kinfos(mib []int32) ([]kinfoProc, error) {
	// First call: get size
	var size uintptr
	_, _, errno := syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		uintptr(len(mib)),
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

	// Allocate buffer and fetch data
	buf := make([]byte, size)
	_, _, errno = syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		uintptr(len(mib)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
		0,
	)
	if errno != 0 {
		return nil, errno
	}

	buf = buf[:size]
	return parseKinfos(buf)
}

// kinfo_proc struct size on darwin/amd64 and darwin/arm64
const kinfoSize = 648

// parseKinfos extracts our fields from raw kinfo_proc structs.
func parseKinfos(buf []byte) ([]kinfoProc, error) {
	n := len(buf) / kinfoSize
	procs := make([]kinfoProc, 0, n)
	for i := 0; i < n; i++ {
		data := buf[i*kinfoSize : (i+1)*kinfoSize]
		var kp kinfoProc
		// kp_proc.p_pid at offset 40
		kp.Pid = int32(binary.LittleEndian.Uint32(data[40:44]))
		// kp_proc.p_comm at offset 243 (16 bytes)
		copy(kp.Comm[:], data[243:259])
		// kp_eproc.e_ppid at offset 560
		kp.Ppid = int32(binary.LittleEndian.Uint32(data[560:564]))
		// kp_eproc.e_ucred.cr_uid at offset 392
		kp.Uid = binary.LittleEndian.Uint32(data[392:396])
		if kp.Pid == 0 && kp.Ppid == 0 && kp.Comm[0] == 0 {
			continue // skip empty entries
		}
		procs = append(procs, kp)
	}
	return procs, nil
}

// procPidpath gets the full path of a process using __proc_info syscall.
func procPidpath(pid int) (string, error) {
	buf := make([]byte, 4096) // PROC_PIDPATHINFO_MAXSIZE
	// __proc_info(PROC_INFO_CALL_PIDINFO, pid, PROC_PIDPATHINFO, 0, buf, bufsize)
	_, _, errno := syscall.Syscall6(
		336, // SYS_proc_info
		2,   // PROC_INFO_CALL_PIDINFO
		uintptr(pid),
		11, // PROC_PIDPATHINFO
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if errno != 0 {
		return "", errno
	}
	return cstring(buf), nil
}

// procArgs gets command-line arguments using KERN_PROCARGS2.
func procArgs(pid int) ([]string, error) {
	mib := [3]int32{ctlKern, kernProcArgs2, int32(pid)}

	// Get size first
	var size uintptr
	_, _, errno := syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		3,
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
		3,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
		0,
	)
	if errno != 0 {
		return nil, errno
	}
	buf = buf[:size]

	return parseProcArgs2(buf), nil
}

// parseProcArgs2 parses KERN_PROCARGS2 output.
// Format: argc (int32) + exec_path + NULs + argv[0] + NUL + argv[1] + NUL ...
func parseProcArgs2(buf []byte) []string {
	if len(buf) < 4 {
		return nil
	}
	argc := int(binary.LittleEndian.Uint32(buf[:4]))
	buf = buf[4:]

	// Skip exec_path (null-terminated)
	idx := bytes.IndexByte(buf, 0)
	if idx < 0 {
		return nil
	}
	buf = buf[idx:]

	// Skip trailing NUL padding
	for len(buf) > 0 && buf[0] == 0 {
		buf = buf[1:]
	}

	// Parse argv
	args := make([]string, 0, argc)
	for i := 0; i < argc && len(buf) > 0; i++ {
		idx = bytes.IndexByte(buf, 0)
		if idx < 0 {
			args = append(args, string(buf))
			break
		}
		args = append(args, string(buf[:idx]))
		buf = buf[idx+1:]
	}
	return args
}

func cstring(b []byte) string {
	idx := bytes.IndexByte(b, 0)
	if idx < 0 {
		return string(b)
	}
	return string(b[:idx])
}

func basename(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

func lookupUID(uid uint32) string {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}
