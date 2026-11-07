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
)

type Route struct {
	Prefix string
	Target string
	Strip  string
}

type route struct {
	prefix string
	strip  string
	proxy  *httputil.ReverseProxy
}

type Proxy struct {
	routes  []route
	timeout time.Duration
}

func New(routes []Route, timeout time.Duration) (*Proxy, error) {
	p := &Proxy{timeout: timeout}
	for _, rt := range routes {
		target, err := url.Parse(rt.Target)
		if err != nil {
			return nil, fmt.Errorf("invalid target %q for prefix %q: %w", rt.Target, rt.Prefix, err)
		}
		if target.Scheme == "" || target.Host == "" {
			return nil, fmt.Errorf("target %q for prefix %q must be an absolute URL", rt.Target, rt.Prefix)
		}

		prefix := rt.Prefix
		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		strip := rt.Strip
		if strip == "" {
			strip = prefix
		}

		p.routes = append(p.routes, route{
			prefix: prefix,
			strip:  strip,
			proxy: &httputil.ReverseProxy{
				Rewrite: func(pr *httputil.ProxyRequest) {
					pr.SetURL(target)
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
					pr.Out.Host = target.Host
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
			if p.timeout > 0 {
				ctx, cancel := context.WithTimeout(r.Context(), p.timeout)
				defer cancel()
				r = r.WithContext(ctx)
			}
			rt.proxy.ServeHTTP(w, r)
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
