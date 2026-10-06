package job

import (
	"log"
	"time"

	"github.com/we1bboard/we1bboard/internal/clientonline"
	"github.com/we1bboard/we1bboard/internal/web/controller"
	"github.com/we1bboard/we1bboard/internal/web/runtime"
	"github.com/we1bboard/we1bboard/internal/xray"
)

func Start(hub *runtime.Hub) {
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			hub.PingNodes()
		}
	}()
	log.Println("[job] node heartbeat started")
}

// StartAccessLogSampler periodically samples xray access.log for online status and client IPs.
func StartAccessLogSampler(xrayMgr *xray.Manager) {
	if xrayMgr == nil {
		return
	}
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		// Run once immediately so the panel has data without waiting a tick.
		clientonline.SampleAccessLog(xrayMgr.AccessLogPath(), 500)
		for range t.C {
			clientonline.SampleAccessLog(xrayMgr.AccessLogPath(), 500)
		}
	}()
	log.Println("[job] access log sampler started")
}

// StartOutboundSubRefresh periodically refreshes outbound subscriptions with IntervalMin > 0.
func StartOutboundSubRefresh(api *controller.API) {
	go func() {
		t := time.NewTicker(1 * time.Minute)
		defer t.Stop()
		for range t.C {
			api.RefreshDueOutboundSubs()
		}
	}()
	log.Println("[job] outbound-sub refresh started")
}
