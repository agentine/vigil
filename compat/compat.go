// Package compat provides a go-ps compatible API for drop-in migration
// from github.com/mitchellh/go-ps.
package compat

import (
	"context"

	"github.com/agentine/vigil"
)

// Process mirrors the mitchellh/go-ps Process interface.
type Process interface {
	// Pid returns the process ID.
	Pid() int
	// PPid returns the parent process ID.
	PPid() int
	// Executable returns the executable name of the process.
	Executable() string
}

type process struct {
	p vigil.Process
}

func (p *process) Pid() int           { return p.p.PID }
func (p *process) PPid() int          { return p.p.PPID }
func (p *process) Executable() string { return p.p.Executable }

// Processes returns all processes (go-ps compatible).
func Processes() ([]Process, error) {
	procs, err := vigil.List(context.Background())
	if err != nil {
		return nil, err
	}
	result := make([]Process, len(procs))
	for i := range procs {
		result[i] = &process{p: procs[i]}
	}
	return result, nil
}

// FindProcess finds a process by PID (go-ps compatible).
// Returns nil, nil if the process is not found.
func FindProcess(pid int) (Process, error) {
	p, err := vigil.Find(context.Background(), pid)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, nil
	}
	return &process{p: *p}, nil
}
