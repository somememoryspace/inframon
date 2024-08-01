package connectors

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
)

func PingHTTP(address string, service string, skipVerify bool) (int, error) {
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		return 0, fmt.Errorf("invalid http address prefix :: address[%s]", address)
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipVerify},
		},
	}
	resp, err := httpClient.Get(address)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 204 {
		return resp.StatusCode, fmt.Errorf("received non-success :: code[%d]", resp.StatusCode)
	}
	return resp.StatusCode, nil
}
