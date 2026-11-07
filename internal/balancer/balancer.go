package balancer

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"

	"gogate/internal/registry"
)

type Strategy interface {
	Pick(service string, instances []*registry.Instance) *registry.Instance
}

type Random struct{}

func (Random) Pick(_ string, instances []*registry.Instance) *registry.Instance {
	if len(instances) == 0 {
		return nil
	}
	return instances[rand.IntN(len(instances))]
}

type RoundRobin struct {
	counters sync.Map // service name -> *atomic.Uint64
}

func (rr *RoundRobin) counter(service string) *atomic.Uint64 {
	v, _ := rr.counters.LoadOrStore(service, new(atomic.Uint64))
	return v.(*atomic.Uint64)
}

func (rr *RoundRobin) Pick(service string, instances []*registry.Instance) *registry.Instance {
	if len(instances) == 0 {
		return nil
	}
	n := rr.counter(service).Add(1)
	return instances[int(n-1)%len(instances)]
}

func New(name string) (Strategy, error) {
	switch name {
	case "roundrobin", "rr":
		return &RoundRobin{}, nil
	case "random":
		return Random{}, nil
	default:
		return nil, fmt.Errorf("unknown load balancing strategy %q", name)
	}
}
