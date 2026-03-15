//go:build solaris

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
	"strings"
)

// psinfo struct fields (Solaris /proc/PID/psinfo).
// Struct size is 416 bytes on Solaris amd64.
const (
	psinfoSize    = 416
	offPrPid      = 8   // pr_pid (int32)
	offPrPpid     = 12  // pr_ppid (int32)
	offPrUid      = 24  // pr_uid (uint32)
	offPrFname    = 84  // pr_fname (PRFNSZ=16 bytes)
	offPrPsargs   = 100 // pr_psargs (PRARGSZ=80 bytes)
)

func list(ctx context.Context) ([]Process, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("vigil: reading /proc: %w", err)
	}
	var procs []Process
	for _, entry := range entries {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		p, err := readPsinfo(pid)
		if err != nil {
			continue // process may have exited
		}
		procs = append(procs, *p)
	}
	return procs, nil
}

func find(ctx context.Context, pid int) (*Process, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	p, err := readPsinfo(pid)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

func readPsinfo(pid int) (*Process, error) {
	dir := filepath.Join("/proc", strconv.Itoa(pid))

	data, err := os.ReadFile(filepath.Join(dir, "psinfo"))
	if err != nil {
		return nil, err
	}
	if len(data) < psinfoSize {
		return nil, fmt.Errorf("vigil: psinfo too short for pid %d", pid)
	}

	p := &Process{
		PID:        int(int32(binary.LittleEndian.Uint32(data[offPrPid : offPrPid+4]))),
		PPID:       int(int32(binary.LittleEndian.Uint32(data[offPrPpid : offPrPpid+4]))),
		Executable: cstring(data[offPrFname : offPrFname+16]),
	}

	uid := binary.LittleEndian.Uint32(data[offPrUid : offPrUid+4])
	p.User = lookupUID(uid)

	// Parse psargs for args (space-separated, truncated at 80 chars)
	psargs := cstring(data[offPrPsargs : offPrPsargs+80])
	if psargs != "" {
		p.Args = strings.Fields(psargs)
	}

	// Full path via /proc/PID/path/a.out
	link, err := os.Readlink(filepath.Join(dir, "path", "a.out"))
	if err == nil {
		p.Path = link
		// Use full basename if pr_fname was truncated
		base := filepath.Base(link)
		if len(base) > len(p.Executable) {
			p.Executable = base
		}
	}

	// Better args from /proc/PID/cmdline if available
	cmdline, err := os.ReadFile(filepath.Join(dir, "cmdline"))
	if err == nil && len(cmdline) > 0 {
		args := parseCmdline(cmdline)
		if len(args) > 0 {
			p.Args = args
		}
	}

	return p, nil
}

func parseCmdline(data []byte) []string {
	data = bytes.TrimRight(data, "\x00")
	if len(data) == 0 {
		return nil
	}
	parts := bytes.Split(data, []byte{0})
	args := make([]string, len(parts))
	for i, part := range parts {
		args[i] = string(part)
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

func lookupUID(uid uint32) string {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return strconv.FormatUint(uint64(uid), 10)
	}
	return u.Username
}
