package connectors

import (
	"fmt"
	"github.com/prometheus-community/pro-bing"
	"time"
)

func PingICMP(address string, privileged bool, retryBuffer int, failureTimeout int) (time.Duration, error) {
	var lastErr error
	for attempt := 0; attempt <= retryBuffer; attempt++ {
		latency, err := performICMPPing(address, privileged, failureTimeout)
		if err == nil {
			return latency, nil
		}
		lastErr = err
		if attempt < retryBuffer {
			time.Sleep(time.Second * time.Duration(attempt+1))
		}
	}
	return 0, fmt.Errorf("icmp ping failed after %d retries: %v", retryBuffer, lastErr)
}

func performICMPPing(address string, privileged bool, failureTimeout int) (time.Duration, error) {
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
	pinger.Timeout = time.Duration(failureTimeout) * time.Second
	err = pinger.Run()
	if err != nil {
		return 0, err
	}
	stats := pinger.Statistics()
	if stats.AvgRtt == 0 {
		return 0, fmt.Errorf("failed to create ping for address: %s", address)
	}
	return stats.AvgRtt, nil
}
