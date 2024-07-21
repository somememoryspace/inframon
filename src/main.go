package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-ping/ping"
	"github.com/somememoryspace/inframon/src/utils"
)

var (
	CONFIG = utils.ParseConfig("./config/config.yaml")
	LOGGER = utils.SetupLogger(CONFIG.Configuration.LogFileDirectory, CONFIG.Configuration.LogFileName)

	ICMPHEALTH = make(map[string]bool)
	HTTPHEALTH = make(map[string]bool)
	STATUSMAP  = make(map[string]bool)

	STDOUT            = CONFIG.Configuration.Stdout
	DISCORDISABLED    = CONFIG.Configuration.DiscordWebHookDisable
	DISCORDWEBHOOKURL = CONFIG.Configuration.DiscordWebHookURL
	HEALTHCHECKTIME   = CONFIG.Configuration.HealthCheckTimeout

	MUTEX sync.Mutex
)

func setICMPHealth(ip string, value bool) {
	MUTEX.Lock()
	defer MUTEX.Unlock()
	ICMPHEALTH[ip] = value
}

func getICMPHealth(ip string) bool {
	MUTEX.Lock()
	defer MUTEX.Unlock()
	return ICMPHEALTH[ip]
}

func setHTTPHealth(fqdn string, value bool) {
	MUTEX.Lock()
	defer MUTEX.Unlock()
	HTTPHEALTH[fqdn] = value
}

func getHTTPHealth(fqdn string) bool {
	MUTEX.Lock()
	defer MUTEX.Unlock()
	return HTTPHEALTH[fqdn]
}

func pingICMP(address string, retryBuffer int) time.Duration {
	consecutiveFailures := 0

	for {
		pinger, err := ping.NewPinger(address)
		if err != nil {
			utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("icmp ::::: could not connect to address: (%s)", address), "error", STDOUT)
			setICMPHealth(address, false)
		} else {
			pinger.Count = 1
			pinger.Timeout = 2 * time.Second
			err = pinger.Run()
			if err != nil {
				utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("icmp ::::: could not connect to address: (%s)", address), "error", STDOUT)
				setICMPHealth(address, false)
			} else {
				consecutiveFailures = 0
				stats := pinger.Statistics()
				return stats.AvgRtt
			}
		}

		consecutiveFailures++
		if consecutiveFailures > retryBuffer {
			return 0
		}
		time.Sleep(1 * time.Second)
	}
}

func pingTaskICMP(ip, hostname string, tags []string, timeout int, wg *sync.WaitGroup, retrybuffer int) {
	defer wg.Done()
	for {
		latency := pingICMP(ip, retrybuffer)
		if latency == 0 {
			if setStatusAndCheckIfAlreadySent(hostname, "ICMP [BROKEN]", false) {
				discordWebhookFailure(hostname, "N/A", tags, "ICMP [BROKEN]")
			}
		} else {
			if setStatusAndCheckIfAlreadySent(hostname, fmt.Sprintf("ICMP latency %v", latency), true) {
				discordWebhookSuccess(hostname, "N/A", tags, fmt.Sprintf("ICMP latency %v", latency))
			}
		}
		utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("icmp ::::: %s (%s) (%s) ::::: latency %v", hostname, ip, utils.FormatTags(tags), latency), "info", STDOUT)
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}

func pingHTTP(hostname string, retryBuffer int, skipverify bool) (int, error) {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipverify},
		},
	}
	consecutiveFailures := 0
	for {
		if !strings.HasPrefix(hostname, "http://") && !strings.HasPrefix(hostname, "https://") {
			setHTTPHealth(hostname, false)
			return 0, nil
		}
		resp, err := httpClient.Get(hostname)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("http ::::: timeout connecting to address: (%s)", hostname), "error", STDOUT)
			} else {
				utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("http ::::: could not connect to address: (%s)", hostname), "error", STDOUT)
			}
			setHTTPHealth(hostname, false)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 204 {
				utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("http ::::: received non-success status code (%d) for (%s)", resp.StatusCode, hostname), "error", STDOUT)
				setHTTPHealth(hostname, false)
				return resp.StatusCode, fmt.Errorf("http error: %s, ::::: [%s]", hostname, resp.Status)
			}
			setHTTPHealth(hostname, true)
			consecutiveFailures = 0
			return resp.StatusCode, nil
		}

		consecutiveFailures++
		if consecutiveFailures > retryBuffer {
			return 0, fmt.Errorf("maximum retries reached for %s", hostname)
		}
		time.Sleep(1 * time.Second)
	}
}

func pingTaskHTTP(fqdn string, tags []string, timeout int, wg *sync.WaitGroup, retrybuffer int, skipverify bool) {
	defer wg.Done()
	for {
		respCode, err := pingHTTP(fqdn, retrybuffer, skipverify)
		if err != nil && respCode != 0 {
			if setStatusAndCheckIfAlreadySent(fqdn, fmt.Sprintf("HTTP [%v]", respCode), false) {
				discordWebhookFailure("N/A", fqdn, tags, fmt.Sprintf("HTTP [%v]", respCode))
			}
		}
		if respCode == 0 {
			if setStatusAndCheckIfAlreadySent(fqdn, "HTTP [UNREACHABLE]", false) {
				discordWebhookFailure("N/A", fqdn, tags, "HTTP [UNREACHABLE]")
			}
		} else {
			if setStatusAndCheckIfAlreadySent(fqdn, fmt.Sprintf("HTTP [%v]", respCode), true) {
				discordWebhookSuccess("N/A", fqdn, tags, fmt.Sprintf("HTTP [%v]", respCode))
			}
		}
		utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("http ::::: %s (%s) ::: [%v]", fqdn, utils.FormatTags(tags), respCode), "info", STDOUT)
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}

func setStatusAndCheckIfAlreadySent(key, status string, currentStatus bool) bool {
	MUTEX.Lock()
	defer MUTEX.Unlock()
	prevStatus, exists := STATUSMAP[key]
	if exists && prevStatus == currentStatus {
		return false
	}
	STATUSMAP[key] = currentStatus
	return true
}

func discordWebhookFailure(hostname string, fqdn string, tags []string, errorMessage string) {
	embed := utils.DiscordEmbed{
		Title:       "Service Unreachable",
		Description: errorMessage,
		Color:       0xFF0000,
		Fields: []utils.DiscordField{
			{
				Name:   "Hostname",
				Value:  hostname,
				Inline: true,
			},
			{
				Name:   "FQDN",
				Value:  fqdn,
				Inline: true,
			},
			{
				Name:   "Date",
				Value:  time.Now().Format("2006-01-02"),
				Inline: true,
			},
			{
				Name:   "Time",
				Value:  time.Now().Format("15:04:05"),
				Inline: true,
			},
			{
				Name:   "Tags",
				Value:  strings.Join(tags, ", "),
				Inline: true,
			},
		},
	}
	message := utils.Message{
		Embeds: []utils.DiscordEmbed{embed},
	}
	sendDiscordWebhook(message)
}

func discordWebhookSuccess(hostname string, fqdn string, tags []string, successMessage string) {
	embed := utils.DiscordEmbed{
		Title:       "Service Active",
		Description: successMessage,
		Color:       0x00FF00,
		Fields: []utils.DiscordField{
			{
				Name:   "Hostname",
				Value:  hostname,
				Inline: true,
			},
			{
				Name:   "FQDN",
				Value:  fqdn,
				Inline: true,
			},
			{
				Name:   "Date",
				Value:  time.Now().Format("2006-01-02"),
				Inline: true,
			},
			{
				Name:   "Time",
				Value:  time.Now().Format("15:04:05"),
				Inline: true,
			},
			{
				Name:   "Tags",
				Value:  strings.Join(tags, ", "),
				Inline: true,
			},
		},
	}
	message := utils.Message{
		Embeds: []utils.DiscordEmbed{embed},
	}
	sendDiscordWebhook(message)
}

func sendDiscordWebhook(message utils.Message) {
	if DISCORDISABLED {
		utils.ConsoleAndLoggerOutput(LOGGER, "syst ::::: discord webhook service disabled", "info", STDOUT)
		return
	}
	jsonPayload, err := json.Marshal(message)
	if err != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("syst ::::: error marshalling json: %v", err), "error", STDOUT)
		return
	}
	resp, err := http.Post(DISCORDWEBHOOKURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("syst ::::: error sending discord webhook status code: [%d]", resp.StatusCode), "error", STDOUT)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 {
		utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("syst ::::: error sending discord webhook status code: [%d]", resp.StatusCode), "error", STDOUT)
		return
	}
	utils.ConsoleAndLoggerOutput(LOGGER, "syst ::::: message sent successfully to discord webhook", "info", STDOUT)
}

func healthCheck(timeout int, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		for ip := range ICMPHEALTH {
			if !getICMPHealth(ip) {
				utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("icmp ::::: health for %s ::::: failing", ip), "info", STDOUT)
			}
		}
		for fqdn := range HTTPHEALTH {
			if !getHTTPHealth(fqdn) {
				utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("http ::::: health for %s ::::: failing", fqdn), "info", STDOUT)
			}
		}
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}

func main() {
	utils.ConsoleAndLoggerOutput(LOGGER, "syst ::::: runtime start", "info", STDOUT)
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	var wg sync.WaitGroup
	for _, icmp := range CONFIG.ICMP {
		setICMPHealth(icmp.IP, true)
	}
	for _, http := range CONFIG.HTTP {
		setHTTPHealth(http.FQDN, true)
	}
	for _, icmp := range CONFIG.ICMP {
		wg.Add(1)
		go pingTaskICMP(icmp.IP, icmp.Hostname, icmp.Tags, icmp.Timeout, &wg, icmp.RetryBuffer)
	}
	for _, http := range CONFIG.HTTP {
		wg.Add(1)
		go pingTaskHTTP(http.FQDN, http.Tags, http.Timeout, &wg, http.RetryBuffer, http.SkipVerify)
	}
	wg.Add(1)
	go healthCheck(HEALTHCHECKTIME, &wg)
	<-signalChannel
	utils.ConsoleAndLoggerOutput(LOGGER, "syst ::::: termination signal received. exiting...", "info", STDOUT)
}
