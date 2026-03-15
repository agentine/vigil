package vigil

// Process represents a single OS process.
type Process struct {
	PID        int      // process ID
	PPID       int      // parent process ID
	Executable string   // full executable name (not truncated)
	Path       string   // full executable path (when available)
	Args       []string // command-line arguments (when available)
	User       string   // process owner (when available)
}
