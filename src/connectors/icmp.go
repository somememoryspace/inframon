package connectors

import (
	"time"

	"github.com/go-ping/ping"
)

func PingICMP(address string) time.Duration {
	pinger, err := ping.NewPinger(address)
	if err != nil {
		return 0
	}
	pinger.Count = 1
	pinger.Timeout = 2 * time.Second
	err = pinger.Run()
	if err != nil {
		return 0
	}
	stats := pinger.Statistics()
	return stats.AvgRtt
}
