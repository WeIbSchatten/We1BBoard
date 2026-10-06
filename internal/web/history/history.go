// Package history keeps a lightweight in-memory ring of host metrics samples.
package history

import (
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	psnet "github.com/shirou/gopsutil/v4/net"
)

const ringSize = 60

// Snapshot is returned by GET /server/history.
type Snapshot struct {
	CPU []float64 `json:"cpu"`
	Mem []float64 `json:"mem"`
	TCP []int     `json:"tcp"`
	UDP []int     `json:"udp"`
}

var (
	mu   sync.RWMutex
	cpuR = make([]float64, 0, ringSize)
	memR = make([]float64, 0, ringSize)
	tcpR = make([]int, 0, ringSize)
	udpR = make([]int, 0, ringSize)
	once sync.Once
)

// Start launches the 10s sampler (idempotent).
func Start() {
	once.Do(func() {
		go loop()
	})
}

func loop() {
	// Prime CPU percent (first call often returns 0).
	_, _ = cpu.Percent(0, false)
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	sample()
	for range t.C {
		sample()
	}
}

func sample() {
	var cpuPct float64
	if pcts, err := cpu.Percent(0, false); err == nil && len(pcts) > 0 {
		cpuPct = pcts[0]
	}
	memPct := 0.0
	if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
		memPct = vm.UsedPercent
	}
	tcpCount, udpCount := 0, 0
	if conns, err := psnet.Connections("tcp"); err == nil {
		tcpCount = len(conns)
	}
	if conns, err := psnet.Connections("udp"); err == nil {
		udpCount = len(conns)
	}
	mu.Lock()
	defer mu.Unlock()
	cpuR = pushF(cpuR, cpuPct)
	memR = pushF(memR, memPct)
	tcpR = pushI(tcpR, tcpCount)
	udpR = pushI(udpR, udpCount)
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

// Get returns copies of the current rings.
func Get() Snapshot {
	mu.RLock()
	defer mu.RUnlock()
	return Snapshot{
		CPU: append([]float64(nil), cpuR...),
		Mem: append([]float64(nil), memR...),
		TCP: append([]int(nil), tcpR...),
		UDP: append([]int(nil), udpR...),
	}
}
