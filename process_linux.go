//go:build linux

package vigil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
			continue // not a PID directory
		}
		p, err := readProc(pid)
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
	p, err := readProc(pid)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

func readProc(pid int) (*Process, error) {
	dir := filepath.Join("/proc", strconv.Itoa(pid))

	p := &Process{PID: pid}

	// Read /proc/PID/stat for PPID and comm
	statData, err := os.ReadFile(filepath.Join(dir, "stat"))
	if err != nil {
		return nil, err
	}
	parseStat(p, statData)

	// Read /proc/PID/exe for full path
	exe, err := os.Readlink(filepath.Join(dir, "exe"))
	if err == nil {
		// Remove " (deleted)" suffix if present
		exe = strings.TrimSuffix(exe, " (deleted)")
		p.Path = exe
	}

	// Read /proc/PID/cmdline for args (and full executable name)
	cmdlineData, err := os.ReadFile(filepath.Join(dir, "cmdline"))
	if err == nil && len(cmdlineData) > 0 {
		args := parseCmdline(cmdlineData)
		if len(args) > 0 {
			// Use full executable name from cmdline if comm was truncated
			base := filepath.Base(args[0])
			if len(base) > len(p.Executable) {
				p.Executable = base
			}
			p.Args = args
		}
	}

	// Read /proc/PID/status for UID → username
	statusData, err := os.ReadFile(filepath.Join(dir, "status"))
	if err == nil {
		p.User = parseStatusUID(statusData)
	}

	return p, nil
}

// parseStat parses /proc/PID/stat to extract PPID and comm.
// Format: pid (comm) state ppid ...
func parseStat(p *Process, data []byte) {
	// Find comm between first '(' and last ')'
	start := bytes.IndexByte(data, '(')
	end := bytes.LastIndexByte(data, ')')
	if start < 0 || end < 0 || end <= start {
		return
	}
	p.Executable = string(data[start+1 : end])

	// Fields after ')' are space-separated: state ppid ...
	rest := data[end+2:] // skip ") "
	fields := bytes.Fields(rest)
	if len(fields) >= 2 {
		ppid, err := strconv.Atoi(string(fields[1]))
		if err == nil {
			p.PPID = ppid
		}
	}
}

// parseCmdline splits NUL-delimited /proc/PID/cmdline into args.
func parseCmdline(data []byte) []string {
	// Remove trailing NUL bytes
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

// parseStatusUID extracts the effective UID from /proc/PID/status
// and looks up the username.
func parseStatusUID(data []byte) string {
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if bytes.HasPrefix(line, []byte("Uid:")) {
			fields := bytes.Fields(line)
			// Fields: Uid: real effective saved fs
			if len(fields) >= 3 {
				return lookupUser(string(fields[2])) // effective UID
			}
		}
	}
	return ""
}

// lookupUser resolves a numeric UID to a username by reading /etc/passwd.
func lookupUser(uid string) string {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return uid
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) >= 3 && fields[2] == uid {
			return fields[0]
		}
	}
	return uid
}
