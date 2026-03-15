package compat_test

import (
	"os"
	"testing"

	"github.com/agentine/vigil/compat"
)

func TestProcesses(t *testing.T) {
	procs, err := compat.Processes()
	if err != nil {
		t.Fatalf("Processes() error: %v", err)
	}
	if len(procs) == 0 {
		t.Fatal("Processes() returned no processes")
	}
	t.Logf("Processes() returned %d processes", len(procs))

	// Verify interface methods work
	for _, p := range procs {
		if p.Pid() < 0 {
			t.Errorf("negative PID: %d", p.Pid())
		}
	}
}

func TestProcessesContainsSelf(t *testing.T) {
	procs, err := compat.Processes()
	if err != nil {
		t.Fatalf("Processes() error: %v", err)
	}
	pid := os.Getpid()
	found := false
	for _, p := range procs {
		if p.Pid() == pid {
			found = true
			if p.Executable() == "" {
				t.Error("current process has empty Executable()")
			}
			if p.PPid() <= 0 {
				t.Errorf("current process PPid() = %d", p.PPid())
			}
			t.Logf("self: Pid=%d PPid=%d Exe=%q", p.Pid(), p.PPid(), p.Executable())
			break
		}
	}
	if !found {
		t.Errorf("current process (PID %d) not in Processes()", pid)
	}
}

func TestFindProcess(t *testing.T) {
	pid := os.Getpid()
	p, err := compat.FindProcess(pid)
	if err != nil {
		t.Fatalf("FindProcess(%d) error: %v", pid, err)
	}
	if p == nil {
		t.Fatalf("FindProcess(%d) returned nil", pid)
	}
	if p.Pid() != pid {
		t.Errorf("FindProcess(%d).Pid() = %d", pid, p.Pid())
	}
	if p.Executable() == "" {
		t.Error("FindProcess().Executable() is empty")
	}
}

func TestFindProcessNotExist(t *testing.T) {
	p, err := compat.FindProcess(999999999)
	if err != nil {
		t.Fatalf("FindProcess(999999999) error: %v", err)
	}
	if p != nil {
		t.Errorf("FindProcess(999999999) should return nil, got PID %d", p.Pid())
	}
}

func TestProcessInterface(t *testing.T) {
	// Verify the Process interface matches go-ps API
	pid := os.Getpid()
	p, err := compat.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("process not found")
	}

	// These are the exact methods from mitchellh/go-ps Process interface
	var _ int = p.Pid()
	var _ int = p.PPid()
	var _ string = p.Executable()
}
