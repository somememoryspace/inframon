package connectors

import (
	"fmt"
	"github.com/prometheus-community/pro-bing"
	"time"
)

func PingICMP(address string, privileged bool, retryBuffer int, timeout int) (time.Duration, error) {
	var lastErr error
	for attempt := 0; attempt <= retryBuffer; attempt++ {
		latency, err := performICMPPing(address, privileged, timeout)
		if err == nil {
			return latency, nil
		}
		lastErr = err
		if attempt < retryBuffer {
			time.Sleep(time.Second * time.Duration(attempt+1))
		}
	}
	return 0, fmt.Errorf("ICMP ping failed after %d retries: %v", retryBuffer, lastErr)
}

func performICMPPing(address string, privileged bool, timeout int) (time.Duration, error) {
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
	pinger.Timeout = time.Duration(timeout) * time.Second
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
