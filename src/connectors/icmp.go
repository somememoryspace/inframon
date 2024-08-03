package connectors

import (
	"time"

	"github.com/prometheus-community/pro-bing"
)

func PingICMP(address string, privileged bool) (time.Duration, error) {
	pinger, err := probing.NewPinger(address)
	if err != nil {
		return 0, err
	}
	if privileged {
		pinger.SetPrivileged(true)
	} else {
		pinger.SetNetwork("udp")
	}
	pinger.Count = 1
	pinger.Timeout = 2 * time.Second
	err = pinger.Run()
	if err != nil {
		return 0, err
	}
	stats := pinger.Statistics()
	return stats.AvgRtt, nil
}
