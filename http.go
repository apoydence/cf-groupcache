package cfgroupcache

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang/groupcache"
)

type HTTPPool struct {
	*groupcache.HTTPPool
}

func NewHTTPPool(selfRoute, selfAppInstance string) *HTTPPool {
	return modifier(groupcache.NewHTTPPool(selfRoute + "::" + selfAppInstance))
}

func NewHTTPPoolOpts(selfRoute, selfAppInstance string, o *groupcache.HTTPPoolOptions) *HTTPPool {
	return modifier(groupcache.NewHTTPPoolOpts(selfRoute+"::"+selfAppInstance, o))
}

func modifier(p *groupcache.HTTPPool) *HTTPPool {
	p.Transport = func(groupcache.Context) http.RoundTripper {
		return &requestModifier{}
	}

	return &HTTPPool{p}
}

func (p *HTTPPool) Set(route string, appInstances ...string) {
	var peers []string
	for _, ai := range appInstances {
		peers = append(peers, route+"::"+ai)
	}
	p.HTTPPool.Set(peers...)
}

type requestModifier struct {
}

func (m *requestModifier) RoundTrip(r *http.Request) (*http.Response, error) {
	parts := strings.SplitN(r.URL.Host, "::", 2)
	if len(parts) != 2 || parts[1] == "" {
		panic("malformed request " + r.URL.Host)
	}

	r.Host = parts[0]
	r.URL.Host = parts[0]
	r.Header.Set("X-CF-APP-INSTANCE", parts[1])

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	r = r.WithContext(ctx)

	return http.DefaultTransport.RoundTrip(r)
}
