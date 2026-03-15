package vigil_test

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/agentine/vigil"
)

func TestList(t *testing.T) {
	procs, err := vigil.List(context.Background())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(procs) == 0 {
		t.Fatal("List() returned no processes")
	}
	t.Logf("List() returned %d processes", len(procs))

	// Every process should have a non-negative PID
	for _, p := range procs {
		if p.PID < 0 {
			t.Errorf("process has negative PID: %d", p.PID)
		}
	}
}

func TestListContainsSelf(t *testing.T) {
	procs, err := vigil.List(context.Background())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	pid := os.Getpid()
	found := false
	for _, p := range procs {
		if p.PID == pid {
			found = true
			if p.Executable == "" {
				t.Error("current process has empty Executable")
			}
			t.Logf("self: PID=%d Exe=%q Path=%q User=%q", p.PID, p.Executable, p.Path, p.User)
			break
		}
	}
	if !found {
		t.Errorf("current process (PID %d) not found in List()", pid)
	}
}

func TestListContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := vigil.List(ctx)
	if err == nil {
		t.Error("List() with cancelled context should return error")
	}
}

func TestFind(t *testing.T) {
	pid := os.Getpid()
	p, err := vigil.Find(context.Background(), pid)
	if err != nil {
		t.Fatalf("Find(%d) error: %v", pid, err)
	}
	if p == nil {
		t.Fatalf("Find(%d) returned nil", pid)
	}
	if p.PID != pid {
		t.Errorf("Find(%d).PID = %d", pid, p.PID)
	}
	if p.PPID <= 0 {
		t.Errorf("Find(%d).PPID = %d, want > 0", pid, p.PPID)
	}
	if p.Executable == "" {
		t.Errorf("Find(%d).Executable is empty", pid)
	}
	t.Logf("Find(%d): Exe=%q Path=%q User=%q Args=%v", pid, p.Executable, p.Path, p.User, p.Args)
}

func TestFindPID1(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PID 1 not meaningful on Windows")
	}
	p, err := vigil.Find(context.Background(), 1)
	if err != nil {
		t.Fatalf("Find(1) error: %v", err)
	}
	if p == nil {
		t.Skip("PID 1 not found (may require root)")
	}
	if p.PID != 1 {
		t.Errorf("Find(1).PID = %d", p.PID)
	}
	if p.Executable == "" {
		t.Errorf("Find(1).Executable is empty")
	}
	t.Logf("PID 1: Exe=%q Path=%q User=%q", p.Executable, p.Path, p.User)
}

func TestFindNotExist(t *testing.T) {
	p, err := vigil.Find(context.Background(), 999999999)
	if err != nil {
		t.Fatalf("Find(999999999) error: %v", err)
	}
	if p != nil {
		t.Errorf("Find(999999999) should return nil, got PID %d", p.PID)
	}
}

func TestFindContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := vigil.Find(ctx, os.Getpid())
	if err == nil {
		t.Error("Find() with cancelled context should return error")
	}
}

func TestChildren(t *testing.T) {
	ppid := os.Getppid()
	children, err := vigil.Children(context.Background(), ppid)
	if err != nil {
		t.Fatalf("Children(%d) error: %v", ppid, err)
	}
	// We should be in the children list
	found := false
	pid := os.Getpid()
	for _, c := range children {
		if c.PID == pid {
			found = true
			break
		}
		if c.PPID != ppid {
			t.Errorf("child PID %d has PPID %d, expected %d", c.PID, c.PPID, ppid)
		}
	}
	if !found {
		t.Errorf("current process not found in Children(%d)", ppid)
	}
	t.Logf("Children(%d): %d children", ppid, len(children))
}

func TestChildrenNoChildren(t *testing.T) {
	// Our own process should have no children (test binary is single-process)
	children, err := vigil.Children(context.Background(), os.Getpid())
	if err != nil {
		t.Fatalf("Children() error: %v", err)
	}
	if len(children) != 0 {
		t.Errorf("Children(self) returned %d, want 0", len(children))
	}
}

func TestIter(t *testing.T) {
	count := 0
	pid := os.Getpid()
	found := false
	for p, err := range vigil.Iter(context.Background()) {
		if err != nil {
			t.Fatalf("Iter() error: %v", err)
		}
		count++
		if p.PID == pid {
			found = true
		}
	}
	if count == 0 {
		t.Fatal("Iter() yielded no processes")
	}
	if !found {
		t.Error("Iter() did not yield current process")
	}
	t.Logf("Iter() yielded %d processes", count)
}

func TestIterEarlyBreak(t *testing.T) {
	count := 0
	for _, err := range vigil.Iter(context.Background()) {
		if err != nil {
			t.Fatalf("Iter() error: %v", err)
		}
		count++
		if count >= 3 {
			break
		}
	}
	if count != 3 {
		t.Errorf("early break: got %d, want 3", count)
	}
}

func TestProcessFieldsPopulated(t *testing.T) {
	p, err := vigil.Find(context.Background(), os.Getpid())
	if err != nil {
		t.Fatalf("Find() error: %v", err)
	}
	if p == nil {
		t.Fatal("Find() returned nil for current process")
	}
	if p.PID != os.Getpid() {
		t.Errorf("PID = %d, want %d", p.PID, os.Getpid())
	}
	if p.PPID <= 0 {
		t.Errorf("PPID = %d, want > 0", p.PPID)
	}
	if p.Executable == "" {
		t.Error("Executable is empty")
	}
	if p.Path == "" {
		t.Error("Path is empty")
	}
	if p.User == "" {
		t.Error("User is empty")
	}
}

func TestListAndFindConsistent(t *testing.T) {
	procs, err := vigil.List(context.Background())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(procs) == 0 {
		t.Fatal("List() empty")
	}
	// Pick first process and verify Find returns same data
	target := procs[0]
	p, err := vigil.Find(context.Background(), target.PID)
	if err != nil {
		t.Fatalf("Find(%d) error: %v", target.PID, err)
	}
	// Process may have exited between List and Find
	if p == nil {
		t.Skipf("process %d exited between List and Find", target.PID)
	}
	if p.PID != target.PID {
		t.Errorf("Find PID = %d, want %d", p.PID, target.PID)
	}
}

func BenchmarkList(b *testing.B) {
	ctx := context.Background()
	for range b.N {
		_, _ = vigil.List(ctx)
	}
}

func BenchmarkFind(b *testing.B) {
	ctx := context.Background()
	pid := os.Getpid()
	for range b.N {
		_, _ = vigil.Find(ctx, pid)
	}
}

func BenchmarkIter(b *testing.B) {
	ctx := context.Background()
	for range b.N {
		for _, err := range vigil.Iter(ctx) {
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
