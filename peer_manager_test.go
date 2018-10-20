package cfgroupcache_test

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"testing"
	"time"

	cfgroupcache "github.com/poy/cf-groupcache"
	capi "github.com/poy/go-capi"
	"github.com/poy/onpar"
	. "github.com/poy/onpar/expect"
	. "github.com/poy/onpar/matchers"
)

type TM struct {
	*testing.T
	m               *cfgroupcache.PeerManager
	spyPeerSetter   *spyPeerSetter
	spyStatsFetcher *spyStatsFetcher
}

func TestPeerManager(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TM {
		spyPeerSetter := newSpyPeerSetter()
		spyStatsFetcher := newSpyStatsFetcher()
		return TM{
			T:               t,
			m:               cfgroupcache.NewPeerManager("http://some-route.com", "some-guid", spyPeerSetter, spyStatsFetcher, log.New(ioutil.Discard, "", 0)),
			spyPeerSetter:   spyPeerSetter,
			spyStatsFetcher: spyStatsFetcher,
		}
	})

	o.Spec("sets the routes", func(t TM) {
		t.spyStatsFetcher.stats = []capi.ProcessStats{
			{Index: 9},
			{Index: 11},
		}

		ctx, _ := context.WithTimeout(context.Background(), time.Second)
		t.m.Tick(ctx)

		Expect(t, t.spyStatsFetcher.processGuid).To(Equal("some-guid"))
		Expect(t, t.spyStatsFetcher.ctx).To(Equal(ctx))
		Expect(t, t.spyPeerSetter.route).To(Equal("http://some-route.com"))
		Expect(t, t.spyPeerSetter.appInstances).To(Equal([]string{
			"some-guid:9",
			"some-guid:11",
		}))
	})

	o.Spec("it doesn't set the route if the stats gives an error", func(t TM) {
		t.spyStatsFetcher.err = errors.New("some-error")
		t.m.Tick(context.Background())
		Expect(t, t.spyPeerSetter.route).To(Equal(""))
	})
}

type spyPeerSetter struct {
	route        string
	appInstances []string
}

func newSpyPeerSetter() *spyPeerSetter {
	return &spyPeerSetter{}
}

func (s *spyPeerSetter) Set(route string, appInstances ...string) {
	s.route = route
	s.appInstances = appInstances
}

type spyStatsFetcher struct {
	ctx         context.Context
	processGuid string

	stats []capi.ProcessStats
	err   error
}

func newSpyStatsFetcher() *spyStatsFetcher {
	return &spyStatsFetcher{}
}

func (s *spyStatsFetcher) ProcessStats(ctx context.Context, processGuid string) ([]capi.ProcessStats, error) {
	s.ctx = ctx
	s.processGuid = processGuid

	return s.stats, s.err
}
