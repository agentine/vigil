// Package vigil provides cross-platform process listing and inspection.
//
// It is a drop-in replacement for github.com/mitchellh/go-ps with extended
// functionality including full executable paths, command-line arguments,
// process owner information, and context support.
package vigil

import (
	"context"
	"iter"
)

// List returns all running processes.
func List(ctx context.Context) ([]Process, error) {
	return list(ctx)
}

// Find returns the process with the given PID, or nil if not found.
func Find(ctx context.Context, pid int) (*Process, error) {
	return find(ctx, pid)
}

// Children returns all child processes of the given PID.
func Children(ctx context.Context, pid int) ([]Process, error) {
	procs, err := list(ctx)
	if err != nil {
		return nil, err
	}
	var children []Process
	for _, p := range procs {
		if p.PPID == pid {
			children = append(children, p)
		}
	}
	return children, nil
}

// Iter returns an iterator over all running processes.
// Useful for large process lists where you want to filter without
// loading everything into memory.
func Iter(ctx context.Context) iter.Seq2[Process, error] {
	return func(yield func(Process, error) bool) {
		procs, err := list(ctx)
		if err != nil {
			yield(Process{}, err)
			return
		}
		for _, p := range procs {
			if !yield(p, nil) {
				return
			}
		}
	}
}
