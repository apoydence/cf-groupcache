package cfgroupcache

import (
	"context"
	"fmt"
	"log"

	capi "github.com/apoydence/go-capi"
)

type PeerSetter interface {
	Set(route string, appInstances ...string)
}

type StatsFetcher interface {
	ProcessStats(ctx context.Context, processGuid string) ([]capi.ProcessStats, error)
}

type PeerManager struct {
	route   string
	appGuid string
	s       PeerSetter
	f       StatsFetcher
	log     *log.Logger
}

func NewPeerManager(route, appGuid string, s PeerSetter, f StatsFetcher, log *log.Logger) *PeerManager {
	return &PeerManager{
		route:   route,
		appGuid: appGuid,
		s:       s,
		f:       f,
		log:     log,
	}
}

func (m *PeerManager) Tick(ctx context.Context) {
	stats, err := m.f.ProcessStats(ctx, m.appGuid)
	if err != nil {
		m.log.Printf("failed to fetch stats: %s", err)
		return
	}

	var appInstances []string
	for _, s := range stats {
		appInstances = append(appInstances, fmt.Sprintf("%s:%d", m.appGuid, s.Index))
	}

	m.s.Set(m.route, appInstances...)
}
