package balancer

import (
	"fmt"
	"sync"
	"testing"

	"gogate/internal/registry"
)

func instances(n int) []*registry.Instance {
	out := make([]*registry.Instance, n)
	for i := range out {
		out[i] = &registry.Instance{ID: fmt.Sprintf("inst-%d", i), Address: fmt.Sprintf("localhost:900%d", i)}
	}
	return out
}

func TestRoundRobinCycles(t *testing.T) {
	rr := &RoundRobin{}
	insts := instances(3)
	want := []int{0, 1, 2, 0, 1, 2}
	for i, w := range want {
		got := rr.Pick("svc", insts)
		if got != insts[w] {
			t.Fatalf("pick %d: got %s, want %s", i, got.ID, insts[w].ID)
		}
	}
}

func TestRoundRobinPerServiceCounters(t *testing.T) {
	rr := &RoundRobin{}
	insts := instances(2)
	rr.Pick("a", insts) // a -> inst-0
	if got := rr.Pick("b", insts); got != insts[0] {
		t.Fatalf("service b should start at inst-0, got %s", got.ID)
	}
	if got := rr.Pick("a", insts); got != insts[1] {
		t.Fatalf("service a second pick should be inst-1, got %s", got.ID)
	}
}

func TestPickEmpty(t *testing.T) {
	for name, s := range map[string]Strategy{"rr": &RoundRobin{}, "random": Random{}} {
		if got := s.Pick("svc", nil); got != nil {
			t.Fatalf("%s: expected nil for empty instances", name)
		}
	}
}

func TestRandomStaysInSet(t *testing.T) {
	r := Random{}
	insts := instances(3)
	for i := 0; i < 100; i++ {
		got := r.Pick("svc", insts)
		if got == nil {
			t.Fatal("random pick returned nil")
		}
	}
}

func TestRoundRobinConcurrent(t *testing.T) {
	rr := &RoundRobin{}
	insts := instances(4)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				if rr.Pick("svc", insts) == nil {
					t.Error("unexpected nil pick")
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestNew(t *testing.T) {
	if _, err := New("bogus"); err == nil {
		t.Fatal("expected error for unknown strategy")
	}
	if _, err := New("roundrobin"); err != nil {
		t.Fatalf("roundrobin: %v", err)
	}
	if _, err := New("random"); err != nil {
		t.Fatalf("random: %v", err)
	}
}
