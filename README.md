# vigil

Cross-platform process listing for Go — drop-in replacement for [mitchellh/go-ps](https://github.com/mitchellh/go-ps).

## Features

- **Full executable path** — resolves symlinks and uses platform APIs for the complete path
- **No name truncation** — reads full process name, not just the 15/16-char comm field
- **Command-line arguments** — exposes process args where available
- **Process owner** — reports the username of the process owner
- **Context support** — all functions accept `context.Context` for cancellation
- **Process tree** — `Children()` for parent-child traversal
- **Iterator** — `Iter()` uses Go 1.23 `iter.Seq2` for memory-efficient streaming
- **Compatibility layer** — `vigil/compat` provides go-ps-compatible API for drop-in migration
- **Pure Go** — no CGo, uses only `syscall` on all platforms

## Install

```
go get github.com/agentine/vigil
```

Requires Go 1.23 or later.

## Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/agentine/vigil"
)

func main() {
    // List all processes
    procs, err := vigil.List(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Found %d processes\n", len(procs))

    // Find a specific process
    p, err := vigil.Find(context.Background(), 1)
    if err != nil {
        log.Fatal(err)
    }
    if p != nil {
        fmt.Printf("PID 1: %s (%s)\n", p.Executable, p.Path)
    }

    // Get child processes
    children, err := vigil.Children(context.Background(), 1)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("PID 1 has %d children\n", len(children))

    // Iterate with early exit
    for p, err := range vigil.Iter(context.Background()) {
        if err != nil {
            log.Fatal(err)
        }
        if p.Executable == "sshd" {
            fmt.Printf("Found sshd at PID %d\n", p.PID)
            break
        }
    }
}
```

## API

### Process

```go
type Process struct {
    PID        int      // process ID
    PPID       int      // parent process ID
    Executable string   // full executable name (not truncated)
    Path       string   // full executable path (when available)
    Args       []string // command-line arguments (when available)
    User       string   // process owner (when available)
}
```

### Functions

| Function | Description |
|----------|-------------|
| `List(ctx) ([]Process, error)` | Returns all running processes |
| `Find(ctx, pid) (*Process, error)` | Returns the process with the given PID, or nil |
| `Children(ctx, pid) ([]Process, error)` | Returns all child processes of the given PID |
| `Iter(ctx) iter.Seq2[Process, error]` | Iterator over all running processes |

## Platform Support

| Platform | Process List | Full Path | Args | User |
|----------|-------------|-----------|------|------|
| Linux | /proc | /proc/PID/exe | /proc/PID/cmdline | /proc/PID/status |
| macOS | sysctl | proc_pidpath | KERN_PROCARGS2 | kinfo_proc.ki_uid |
| Windows | CreateToolhelp32Snapshot | QueryFullProcessImageName | — | OpenProcessToken |
| FreeBSD | sysctl | /proc/PID/file | KERN_PROC_ARGS | kinfo_proc.ki_uid |
| Solaris | /proc | /proc/PID/path/a.out | /proc/PID/cmdline | psinfo.pr_uid |

## Migration from go-ps

Replace the import path and use the compatibility layer — no other code changes needed:

```diff
- import ps "github.com/mitchellh/go-ps"
+ import ps "github.com/agentine/vigil/compat"
```

The `compat` package provides the same `Process` interface and functions:

```go
type Process interface {
    Pid() int
    PPid() int
    Executable() string
}

func Processes() ([]Process, error)
func FindProcess(pid int) (Process, error)
```

Or migrate to the full API for access to Path, Args, User, context support, and iterators.

## License

MIT
