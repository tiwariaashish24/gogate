package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"time"

	"gogate/internal/balancer"
	"gogate/internal/registry"
)

type Route struct {
	Prefix   string
	Service  string
	Strip    string
	Strategy balancer.Strategy
}

type instanceKey struct{}

type route struct {
	prefix   string
	strip    string
	service  string
	strategy balancer.Strategy
	proxy    *httputil.ReverseProxy
}

type Proxy struct {
	routes   []route
	registry *registry.Registry
	timeout  time.Duration
}

func New(routes []Route, reg *registry.Registry, timeout time.Duration) (*Proxy, error) {
	p := &Proxy{registry: reg, timeout: timeout}
	for _, rt := range routes {
		if rt.Service == "" {
			return nil, fmt.Errorf("route for prefix %q must specify a service", rt.Prefix)
		}
		prefix := rt.Prefix
		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		strip := rt.Strip
		if strip == "" {
			strip = prefix
		}
		strategy := rt.Strategy
		if strategy == nil {
			strategy = balancer.Random{}
		}

		p.routes = append(p.routes, route{
			prefix:   prefix,
			strip:    strip,
			service:  rt.Service,
			strategy: strategy,
			proxy: &httputil.ReverseProxy{
				Rewrite: func(pr *httputil.ProxyRequest) {
					inst, _ := pr.In.Context().Value(instanceKey{}).(*registry.Instance)
					if inst == nil {
						return
					}
					pr.SetURL(&url.URL{Scheme: "http", Host: inst.Address})
					path := pr.Out.URL.Path
					if strings.HasPrefix(path, strip) {
						path = strings.TrimPrefix(path, strip)
					}
					if path == "" {
						path = "/"
					}
					if !strings.HasPrefix(path, "/") {
						path = "/" + path
					}
					pr.Out.URL.Path = path
					pr.Out.Host = inst.Address
				},
				ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
					log.Printf("proxy error: %s %s: %v", r.Method, r.URL.Path, err)
					writeJSON(w, http.StatusBadGateway, map[string]string{"error": "bad gateway"})
				},
			},
		})
	}

	sort.SliceStable(p.routes, func(i, j int) bool {
		return len(p.routes[i].prefix) > len(p.routes[j].prefix)
	})
	return p, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, rt := range p.routes {
		if r.URL.Path == rt.prefix || strings.HasPrefix(r.URL.Path, rt.prefix+"/") || rt.prefix == "/" {
			inst := rt.strategy.Pick(rt.service, p.registry.HealthyInstances(rt.service))
			if inst == nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{
					"error":   "service unavailable",
					"service": rt.service,
				})
				return
			}
			ctx := context.WithValue(r.Context(), instanceKey{}, inst)
			if p.timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, p.timeout)
				defer cancel()
			}
			rt.proxy.ServeHTTP(w, r.WithContext(ctx))
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "no route for " + r.URL.Path})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
