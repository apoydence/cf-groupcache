package cfgroupcache_test

import (
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	cfgroupcache "github.com/poy/cf-groupcache"
	"github.com/golang/groupcache"
)

var (
	peerAddrs    = flag.String("test_peer_addrs", "", "Comma-separated list of peer addresses; used by TestHTTPPool")
	peerIndex    = flag.Int("test_peer_index", -1, "Index of which peer this child is; used by TestHTTPPool")
	peerChild    = flag.Bool("test_peer_child", false, "True if running as a child process; used by TestHTTPPool")
	cfRouterAddr = flag.String("test_cf_router", "", "The address of the fake CF Router; used by TestHTTPPool")
)

func TestHTTPPool(t *testing.T) {
	if *peerChild {
		beChildForTestHTTPPool()
		os.Exit(0)
	}

	const (
		nChild = 4
		nGets  = 100
	)

	routerAddr, closeRouter := fakeCFRouter(t)
	defer closeRouter()

	var childAddr []string
	for i := 0; i < nChild; i++ {
		childAddr = append(childAddr, pickFreeAddr(t))
	}

	var cmds []*exec.Cmd
	var wg sync.WaitGroup
	for i := 0; i < nChild; i++ {
		cmd := exec.Command(os.Args[0],
			"--test.run=TestHTTPPool",
			"--test_peer_child",
			"--test_peer_addrs="+strings.Join(childAddr, ","),
			"--test_peer_index="+strconv.Itoa(i),
			"--test_cf_router="+routerAddr,
		)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout

		cmds = append(cmds, cmd)
		wg.Add(1)
		if err := cmd.Start(); err != nil {
			t.Fatal("failed to start child process: ", err)
		}
		go awaitAddrReady(t, childAddr[i], &wg)
	}
	defer func() {
		for i := 0; i < nChild; i++ {
			if cmds[i].Process != nil {
				cmds[i].Process.Kill()
			}
		}
	}()
	wg.Wait()

	// Use a dummy self address so that we don't handle gets in-process.
	p := cfgroupcache.NewHTTPPool("should-be-ignored", "should-be-ignored")
	p.Set(routerAddr, childAddr...)

	var called bool
	p.Transport = func(c groupcache.Context) http.RoundTripper {
		called = true
		return http.DefaultTransport
	}
	defer func() {
		if !called {
			t.Fatal("Transport not used")
		}
	}()

	// Dummy getter function. Gets should go to children only.
	// The only time this process will handle a get is when the
	// children can't be contacted for some reason.
	getter := groupcache.GetterFunc(func(ctx groupcache.Context, key string, dest groupcache.Sink) error {
		return errors.New("parent getter called; something's wrong")
	})
	g := groupcache.NewGroup("httpPoolTest", 1<<20, getter)

	for _, key := range testKeys(nGets) {
		var value string
		if err := g.Get(nil, key, groupcache.StringSink(&value)); err != nil {
			t.Fatal(err)
		}
		if suffix := ":" + key; !strings.HasSuffix(value, suffix) {
			t.Errorf("Get(%q) = %q, want value ending in %q", key, value, suffix)
		}
		t.Logf("Get key=%q, value=%q (peer:key)", key, value)
	}
}

func fakeCFRouter(t *testing.T) (string, func()) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appInstance := r.Header.Get("X-CF-APP-INSTANCE")
		if appInstance == "" {
			t.Fatal("missing X-CF-APP-INSTANCE header")
		}

		u, err := url.Parse("http://" + appInstance)
		if err != nil {
			t.Fatal(err)
		}

		httputil.NewSingleHostReverseProxy(u).ServeHTTP(w, r)
	}))

	return server.URL, server.Close
}

func testKeys(n int) (keys []string) {
	keys = make([]string, n)
	for i := range keys {
		keys[i] = strconv.Itoa(i)
	}
	return
}

func beChildForTestHTTPPool() {
	addrs := strings.Split(*peerAddrs, ",")

	p := cfgroupcache.NewHTTPPool(*cfRouterAddr, addrs[*peerIndex])
	p.Set(*cfRouterAddr, addrs...)

	getter := groupcache.GetterFunc(func(ctx groupcache.Context, key string, dest groupcache.Sink) error {
		dest.SetString(strconv.Itoa(*peerIndex) + ":" + key)
		return nil
	})
	groupcache.NewGroup("httpPoolTest", 1<<20, getter)

	log.Fatal(http.ListenAndServe(addrs[*peerIndex], p))
}

// This is racy. Another process could swoop in and steal the port between the
// call to this function and the next listen call. Should be okay though.
// The proper way would be to pass the l.File() as ExtraFiles to the child
// process, and then close your copy once the child starts.
func pickFreeAddr(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func addrToURL(addr []string) []string {
	url := make([]string, len(addr))
	for i := range addr {
		url[i] = "http://" + addr[i]
	}
	return url
}

func awaitAddrReady(t *testing.T, addr string, wg *sync.WaitGroup) {
	defer wg.Done()
	const max = 1 * time.Second
	tries := 0
	for {
		tries++
		c, err := net.Dial("tcp", addr)
		if err == nil {
			c.Close()
			return
		}
		delay := time.Duration(tries) * 25 * time.Millisecond
		if delay > max {
			delay = max
		}
		time.Sleep(delay)
	}
}
