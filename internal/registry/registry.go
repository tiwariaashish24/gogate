package registry

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type Instance struct {
	ID       string `json:"id"`
	Service  string `json:"service"`
	Address  string `json:"address"`
	Healthy  bool   `json:"healthy"`
	Failures int    `json:"failures"`
}

type Registry struct {
	mu       sync.RWMutex
	services map[string]map[string]*Instance
	seq      atomic.Uint64
	client   *http.Client
	interval time.Duration
	maxFails int
}

func New(interval time.Duration, maxFails int) *Registry {
	return &Registry{
		services: make(map[string]map[string]*Instance),
		client:   &http.Client{Timeout: 2 * time.Second},
		interval: interval,
		maxFails: maxFails,
	}
}

func (r *Registry) Register(service, address string) (*Instance, error) {
	if service == "" {
		return nil, fmt.Errorf("service name is required")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: %w", address, err)
	}
	if host == "" {
		host = "localhost"
	}

	inst := &Instance{
		ID:      fmt.Sprintf("%s-%d", service, r.seq.Add(1)),
		Service: service,
		Address: net.JoinHostPort(host, port),
		Healthy: true,
	}

	r.mu.Lock()
	if r.services[service] == nil {
		r.services[service] = make(map[string]*Instance)
	}
	r.services[service][inst.ID] = inst
	r.mu.Unlock()
	return inst, nil
}

func (r *Registry) Deregister(service, id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	instances, ok := r.services[service]
	if !ok {
		return false
	}
	if _, ok := instances[id]; !ok {
		return false
	}
	delete(instances, id)
	if len(instances) == 0 {
		delete(r.services, service)
	}
	return true
}

func (r *Registry) healthyLocked(service string) []*Instance {
	var out []*Instance
	for _, inst := range r.services[service] {
		if inst.Healthy {
			out = append(out, inst)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) HealthyInstances(service string) []*Instance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.healthyLocked(service)
}

func (r *Registry) Pick(service string) *Instance {
	instances := r.HealthyInstances(service)
	if len(instances) == 0 {
		return nil
	}
	return instances[rand.Intn(len(instances))]
}

func (r *Registry) Snapshot() map[string][]*Instance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string][]*Instance, len(r.services))
	for name := range r.services {
		out[name] = r.healthyLocked(name)
	}
	return out
}

func (r *Registry) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.checkAll()
		}
	}
}

func (r *Registry) checkAll() {
	r.mu.RLock()
	var all []*Instance
	for _, instances := range r.services {
		for _, inst := range instances {
			all = append(all, inst)
		}
	}
	r.mu.RUnlock()

	var wg sync.WaitGroup
	for _, inst := range all {
		wg.Add(1)
		go func(inst *Instance) {
			defer wg.Done()
			r.check(inst)
		}(inst)
	}
	wg.Wait()
}

func (r *Registry) check(inst *Instance) {
	healthy := r.ping(inst.Address)

	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.services[inst.Service][inst.ID]
	if !ok {
		return
	}
	if healthy {
		current.Healthy = true
		current.Failures = 0
		return
	}
	current.Failures++
	current.Healthy = false
	if current.Failures >= r.maxFails {
		delete(r.services[inst.Service], inst.ID)
		if len(r.services[inst.Service]) == 0 {
			delete(r.services, inst.Service)
		}
	}
}

func (r *Registry) ping(address string) bool {
	resp, err := r.client.Get("http://" + address + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
