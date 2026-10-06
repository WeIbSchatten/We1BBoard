// Package nodehist keeps per-node online/latency rings from PingNodes.
package nodehist

import (
	"sync"
)

const ringSize = 60

// Snapshot is returned by GET /nodes/:id/history.
type Snapshot struct {
	Online  []int     `json:"online"`
	Latency []float64 `json:"latency"`
}

var (
	mu   sync.RWMutex
	data = map[uint]*rings{}
)

type rings struct {
	online  []int
	latency []float64
}

// Record appends one sample for a node (online 1/0, latency ms).
func Record(nodeID uint, online bool, latencyMs float64) {
	mu.Lock()
	defer mu.Unlock()
	r := data[nodeID]
	if r == nil {
		r = &rings{
			online:  make([]int, 0, ringSize),
			latency: make([]float64, 0, ringSize),
		}
		data[nodeID] = r
	}
	on := 0
	if online {
		on = 1
	}
	r.online = pushI(r.online, on)
	r.latency = pushF(r.latency, latencyMs)
}

// Get returns copies of the rings for a node.
func Get(nodeID uint) Snapshot {
	mu.RLock()
	defer mu.RUnlock()
	r := data[nodeID]
	if r == nil {
		return Snapshot{Online: []int{}, Latency: []float64{}}
	}
	return Snapshot{
		Online:  append([]int(nil), r.online...),
		Latency: append([]float64(nil), r.latency...),
	}
}

func pushF(s []float64, v float64) []float64 {
	if len(s) >= ringSize {
		copy(s, s[1:])
		s[ringSize-1] = v
		return s[:ringSize]
	}
	return append(s, v)
}

func pushI(s []int, v int) []int {
	if len(s) >= ringSize {
		copy(s, s[1:])
		s[ringSize-1] = v
		return s[:ringSize]
	}
	return append(s, v)
}
