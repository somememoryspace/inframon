package connectors

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func PingHTTP(address string, service string, skipVerify bool, retryBuffer int, timeout int) (int, error) {
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		return 0, fmt.Errorf("invalid http address prefix :: address[%s]", address)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipVerify},
		},
	}

	var lastErr error
	for attempt := 0; attempt <= retryBuffer; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", address, nil)
		if err != nil {
			cancel()
			return 0, fmt.Errorf("failed to create request: %v", err)
		}

		resp, err := httpClient.Do(req)
		cancel()

		if err != nil {
			lastErr = err
			if isRetryableError(err) && attempt < retryBuffer {
				fmt.Printf("Retryable error: %v. Retrying (%d/%d)\n", err, attempt+1, retryBuffer)
				time.Sleep(time.Second * time.Duration(attempt+1))
				continue
			}
			return 0, fmt.Errorf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 204 {
			return resp.StatusCode, fmt.Errorf("received non-success :: code[%d]", resp.StatusCode)
		}
		return resp.StatusCode, nil
	}

	return 0, fmt.Errorf("request failed after %d retries: %v", retryBuffer, lastErr)
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if err == context.DeadlineExceeded {
		return true
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	switch err := err.(type) {
	case *net.OpError:
		return err.Op == "dial" || err.Op == "read"
	case *url.Error:
		return err.Timeout() || isRetryableError(err.Err)
	}
	if strings.Contains(err.Error(), "connection refused") {
		return true
	}
	if strings.Contains(err.Error(), "context deadline exceeded") {
		return true
	}
	return false
}
