package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
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
	CONFIG     = utils.ParseConfig("./config/config.yaml")
	STDOUT     = CONFIG.Configuration.Stdout
	LOGGER     = utils.SetupLogger(CONFIG.Configuration.LogFileDirectory, CONFIG.Configuration.LogFileName)
	mutex      sync.Mutex
	ICMPHEALTH = make(map[string]bool)
	HTTPHEALTH = make(map[string]bool)
)

func setICMPHealth(ip string, value bool) {
	mutex.Lock()
	defer mutex.Unlock()
	ICMPHEALTH[ip] = value
}

func getICMPHealth(ip string) bool {
	mutex.Lock()
	defer mutex.Unlock()
	return ICMPHEALTH[ip]
}

func setHTTPHealth(fqdn string, value bool) {
	mutex.Lock()
	defer mutex.Unlock()
	HTTPHEALTH[fqdn] = value
}

func getHTTPHealth(fqdn string) bool {
	mutex.Lock()
	defer mutex.Unlock()
	return HTTPHEALTH[fqdn]
}

func pingICMP(address string, wg *sync.WaitGroup) time.Duration {
	wg.Add(1)
	defer wg.Done()
	pinger, err := ping.NewPinger(address)
	if err != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("icmp ::::: could not connect to address: (%s)", address), "error", STDOUT)
		return 0
	}
	pinger.Count = 1
	pinger.Timeout = 2 * time.Second
	err = pinger.Run()
	if err != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("icmp ::::: could not connect to address: (%s)", address), "error", STDOUT)
		return 0
	}
	stats := pinger.Statistics()
	return stats.AvgRtt
}

func pingTaskICMP(ip string, hostname string, tags []string, retryBuffer int, timeout int, wg *sync.WaitGroup, discordWebhookURL string) {
	wg.Add(1)
	defer wg.Done()
	consecutiveFailures := 0
	for {
		latency := pingICMP(ip, wg)
		if latency == 0 {
			if getICMPHealth(ip) {
				consecutiveFailures++
				if consecutiveFailures > retryBuffer {
					utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("icmp ::::: could not connect to address: (%s)", ip), "error", STDOUT)
					setICMPHealth(ip, false)
					sendToDiscordWebhookFailure(discordWebhookURL, hostname, "N/A", tags, "ICMP [BROKEN]", true)
				}
				time.Sleep(1 * time.Second)
				continue
			}
		} else {
			if !getICMPHealth(ip) {
				setICMPHealth(ip, true)
				sendToDiscordWebhookFailure(discordWebhookURL, hostname, "N/A", tags, "ICMP SUCCESS", false)
			}
			utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("icmp ::::: %s (%s) (%s) ::::: latency %v", hostname, ip, utils.FormatTags(tags), latency), "info", STDOUT)
			time.Sleep(time.Duration(timeout) * time.Second)
		}
		consecutiveFailures = 0
	}
}

func pingHTTP(hostname string, retryBuffer int, skipverify bool, wg *sync.WaitGroup) (int, error) {
	wg.Add(1)
	defer wg.Done()
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipverify},
		},
	}
	if !strings.HasPrefix(hostname, "http://") && !strings.HasPrefix(hostname, "https://") {
		return 0, nil
	}
	resp, err := httpClient.Get(hostname)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 204 {
		utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("http ::::: received non-success status code (%d) for (%s)", resp.StatusCode, hostname), "error", STDOUT)
		return resp.StatusCode, nil
	}
	return resp.StatusCode, nil
}

func pingTaskHTTP(fqdn string, tags []string, timeout int, wg *sync.WaitGroup, discordWebhookURL string, retrybuffer int, skipverify bool) {
	wg.Add(1)
	defer wg.Done()
	consecutiveFailures := 0
	for {
		respCode, err := pingHTTP(fqdn, retrybuffer, skipverify, wg)
		if err != nil || respCode == 0 {
			if getHTTPHealth(fqdn) {
				consecutiveFailures++
				if consecutiveFailures > retrybuffer {
					utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("http ::::: could not connect to host: (%s)", fqdn), "error", STDOUT)
					setHTTPHealth(fqdn, false)
					sendToDiscordWebhookFailure(discordWebhookURL, fqdn, "N/A", tags, "HTTP [BROKEN]", true)
				}
				time.Sleep(1 * time.Second)
				continue
			}
		} else if respCode == 200 || respCode == 201 || respCode == 204 {
			if !getHTTPHealth(fqdn) {
				setICMPHealth(fqdn, true)
				sendToDiscordWebhookFailure(discordWebhookURL, fqdn, "N/A", tags, "HTTP SUCCESS", false)
			}
			utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("http ::::: %s (%s) ::: [%v]", fqdn, utils.FormatTags(tags), respCode), "info", STDOUT)
			time.Sleep(time.Duration(timeout) * time.Second)
		}
		consecutiveFailures = 0
	}
}

func sendToDiscordWebhookFailure(webhookURL string, hostname string, fqdn string, tags []string, messageBody string, typeFailure bool) {
	if typeFailure {
		embed := utils.DiscordEmbed{
			Title:       "Unable to Connect",
			Description: messageBody,
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
					Value:  string(utils.FormatTags(tags)),
					Inline: true,
				},
			},
		}
		message := utils.Message{
			Embeds: []utils.DiscordEmbed{embed},
		}
		jsonPayload, err := json.Marshal(message)
		if err != nil {
			utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("syst ::::: error marshalling json: %v", err), "error", STDOUT)
			return
		}
		resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonPayload))
		if err != nil {
			utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("syst ::::: error sending Discord webhook: %v", err), "error", STDOUT)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != 204 {
			utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("syst ::::: unexpected status code: [%d]", resp.StatusCode), "error", STDOUT)
			return
		}
		utils.ConsoleAndLoggerOutput(LOGGER, "syst ::::: message sent successfully to discord webhook", "info", STDOUT)
	} else {
		embed := utils.DiscordEmbed{
			Title:       "Connecion Success",
			Description: messageBody,
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
					Value:  string(utils.FormatTags(tags)),
					Inline: true,
				},
			},
		}
		message := utils.Message{
			Embeds: []utils.DiscordEmbed{embed},
		}
		jsonPayload, err := json.Marshal(message)
		if err != nil {
			utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("syst ::::: error marshalling json: %v", err), "error", STDOUT)
			return
		}
		resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonPayload))
		if err != nil {
			utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("syst ::::: error sending Discord webhook: %v", err), "error", STDOUT)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != 204 {
			utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("syst ::::: unexpected status code: [%d]", resp.StatusCode), "error", STDOUT)
			return
		}
		utils.ConsoleAndLoggerOutput(LOGGER, "syst ::::: message sent successfully to discord webhook", "info", STDOUT)
	}

}

func healthCheck(timeout int, wg *sync.WaitGroup) {
	for {
		for ip := range ICMPHEALTH {
			if !getICMPHealth(ip) {
				utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("icmp ::::: health for %s ::::: failing", ip), "info", STDOUT)
			} else {
				utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("icmp ::::: health for %s ::::: passing", ip), "info", STDOUT)
			}
		}
		for fqdn := range HTTPHEALTH {
			if !getHTTPHealth(fqdn) {
				utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("http ::::: health for %s ::::: failing", fqdn), "info", STDOUT)
			} else {
				utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("icmp ::::: health for %s ::::: passing", fqdn), "info", STDOUT)
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
	// Setup Health Checks
	for _, icmp := range CONFIG.ICMP {
		setICMPHealth(icmp.IP, true)
	}
	for _, http := range CONFIG.HTTP {
		setHTTPHealth(http.FQDN, true)
	}
	// Run ICMP Checks
	for _, icmp := range CONFIG.ICMP {
		wg.Add(1)
		go pingTaskICMP(icmp.IP, icmp.Hostname, icmp.Tags, icmp.RetryBuffer, icmp.Timeout, &wg, CONFIG.Configuration.DiscordWebHookURL)
	}
	// Run HTTP Checks
	for _, http := range CONFIG.HTTP {
		wg.Add(1)
		go pingTaskHTTP(http.FQDN, http.Tags, http.Timeout, &wg, CONFIG.Configuration.DiscordWebHookURL, http.RetryBuffer, http.SkipVerify)
	}
	// Run HealthChecks
	wg.Add(1)
	go healthCheck(CONFIG.Configuration.HealthCheckTimeout, &wg)
	<-signalChannel
	utils.ConsoleAndLoggerOutput(LOGGER, "syst ::::: termination signal received. exiting...", "info", STDOUT)
}
