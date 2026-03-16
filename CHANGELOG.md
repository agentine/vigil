# Changelog

## v0.1.0

Initial release — cross-platform process listing library for Go, replacing the archived [mitchellh/go-ps](https://github.com/mitchellh/go-ps).

### Features

- **Process struct** with PID, PPID, Executable, Path, Args, and User fields
- **List** / **Find** / **Children** / **Iter** functions with `context.Context` support
- **Platform support:** Linux (`/proc`), macOS (`sysctl` + `proc_pidpath`), Windows (`CreateToolhelp32Snapshot`), FreeBSD (`sysctl`), Solaris (`/proc/psinfo`)
- **Full executable path** — resolves symlinks and platform-specific APIs (fixes go-ps #49)
- **No name truncation** — reads full process name from `/proc/PID/cmdline` on Linux (fixes go-ps #15)
- **Command-line arguments** and **process owner** where available
- **Iterator** — `Iter()` using Go 1.23 `iter.Seq2` for memory-efficient streaming
- **Process tree** — `Children()` for parent-child traversal
- **Compatibility layer** — `vigil/compat` provides drop-in go-ps API (`Processes()`, `FindProcess()`, `Process` interface)
- Pure Go, no CGo, minimal dependencies
