# vigil — Process Listing for Go

**Replaces:** [mitchellh/go-ps](https://github.com/mitchellh/go-ps)
**Package:** `github.com/agentine/vigil`
**Language:** Go

## Why Replace go-ps?

- **Archived:** July 22, 2024. Read-only, no future fixes.
- **Single maintainer:** mitchellh, no longer writing Go.
- **854 importers**, 1.4k stars, 250 forks — significant adoption with no migration path.
- **No dominant fork:** goss-org/go-ps (internal, ~0 outside importers), tklauser/ps (1 importer), keybase/go-ps (internal fork). None have traction.
- **gopsutil is NOT a replacement:** different scope (full psutil port), heavy dependency, different API.
- **Known unfixed issues:** truncated executable names on macOS/Linux (#15), no full executable path (#49), no command-line args, no process owner info.

## Scope

Cross-platform process listing library for Go. Pure Go (no CGo). Minimal dependencies.

## API Design

```go
package vigil

// Process represents a single OS process.
type Process struct {
    PID        int
    PPID       int
    Executable string   // full executable name (not truncated)
    Path       string   // full executable path (when available)
    Args       []string // command-line arguments (when available)
    User       string   // process owner (when available)
}

// List returns all running processes.
func List(ctx context.Context) ([]Process, error)

// Find returns the process with the given PID, or nil if not found.
func Find(ctx context.Context, pid int) (*Process, error)

// Children returns all child processes of the given PID.
func Children(ctx context.Context, pid int) ([]Process, error)

// Iter returns an iterator over all running processes.
// Useful for large process lists where you want to filter without
// loading everything into memory.
func Iter(ctx context.Context) iter.Seq2[Process, error]
```

## Improvements Over go-ps

1. **Full executable path** — resolves `/proc/PID/exe` symlink (Linux), full path via sysctl (macOS), full module path (Windows). Fixes the most-requested feature.
2. **No name truncation** — reads `/proc/PID/comm` AND `/proc/PID/cmdline` to get full name. Fixes known bug.
3. **Command-line arguments** — exposes process args where available.
4. **Process owner** — reports the UID/username of the process owner.
5. **Context support** — all functions accept `context.Context` for cancellation.
6. **Process tree** — `Children()` for parent-child traversal.
7. **Iterator** — `Iter()` uses Go 1.23 `iter.Seq2` for memory-efficient streaming.
8. **Struct return** — concrete `Process` struct instead of interface, simpler to use.
9. **Broader platform support** — Linux, macOS, Windows, FreeBSD, Solaris (same as go-ps + FreeBSD).
10. **Compatibility layer** — `vigil/compat` package provides go-ps-compatible `Process` interface and `Processes()`/`FindProcess()` functions for drop-in migration.

## Platform Implementation

| Platform | Process Listing | Full Path | Args | User |
|----------|----------------|-----------|------|------|
| Linux | /proc/PID/stat | /proc/PID/exe readlink | /proc/PID/cmdline | /proc/PID/status (Uid) |
| macOS | sysctl KERN_PROC | /proc not available, use proc_pidpath via syscall | sysctl KERN_PROCARGS2 | ki_uid from kinfo_proc |
| Windows | CreateToolhelp32Snapshot | QueryFullProcessImageName | CommandLineToArgvW (via NtQueryInformationProcess) | OpenProcessToken + LookupAccountSid |
| FreeBSD | sysctl KERN_PROC | /proc/PID/file readlink | sysctl KERN_PROC_ARGS | ki_uid from kinfo_proc |
| Solaris | /proc/PID/psinfo | /proc/PID/path/a.out readlink | /proc/PID/cmdline | psinfo.pr_uid |

All implementations are pure Go using `syscall`/`golang.org/x/sys` — no CGo.

## Architecture

```
vigil/
├── vigil.go          # Public API (List, Find, Children, Iter)
├── process.go        # Process struct definition
├── process_linux.go  # Linux /proc implementation
├── process_darwin.go # macOS sysctl implementation
├── process_windows.go# Windows API implementation
├── process_freebsd.go# FreeBSD sysctl implementation
├── process_solaris.go# Solaris /proc implementation
├── compat/
│   └── compat.go     # go-ps compatible API (Process interface, Processes(), FindProcess())
├── go.mod
└── vigil_test.go     # Cross-platform test suite
```

## Compatibility Layer

```go
package compat

// Process mirrors the mitchellh/go-ps Process interface.
type Process interface {
    Pid() int
    PPid() int
    Executable() string
}

// Processes returns all processes (go-ps compatible).
func Processes() ([]Process, error)

// FindProcess finds a process by PID (go-ps compatible).
func FindProcess(pid int) (Process, error)
```

Migration path: replace `github.com/mitchellh/go-ps` import with `github.com/agentine/vigil/compat` — no code changes needed.

## Deliverables

1. Core process listing (List, Find) for Linux, macOS, Windows
2. Extended info (Path, Args, User) per platform
3. Children() and Iter() functions
4. FreeBSD and Solaris support
5. Compatibility layer (vigil/compat)
6. Test suite with platform-specific tests
7. README with migration guide
