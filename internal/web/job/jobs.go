package job

import (
	"log"
	"time"

	"github.com/we1bboard/we1bboard/internal/web/controller"
	"github.com/we1bboard/we1bboard/internal/web/runtime"
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
