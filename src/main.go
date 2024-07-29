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
	MUTEX              sync.Mutex
	CONFIG             = utils.ParseConfig("./config/config.yaml")
	LOGGER             = utils.SetupLogger(CONFIG.Configuration.LogFileDirectory, CONFIG.Configuration.LogFileName)
	ICMPHEALTH         = make(map[string]bool)
	HTTPHEALTH         = make(map[string]bool)
	DISCORDDISABLE     = CONFIG.Configuration.DiscordWebHookDisable
	HEALTHCHECKTIMEOUT = CONFIG.Configuration.HealthCheckTimeout
	STDOUT             = CONFIG.Configuration.Stdout
)

func setHealthStatus(m map[string]bool, key string, value bool) {
	MUTEX.Lock()
	defer MUTEX.Unlock()
	m[key] = value
}
func getHealthStatus(m map[string]bool, key string) bool {
	MUTEX.Lock()
	defer MUTEX.Unlock()
	return m[key]
}
func pingICMP(address string) time.Duration {
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
func pingTaskICMP(ip, hostname string, tags []string, retryBuffer, timeout int, wg *sync.WaitGroup, discordWebhookURL string) {
	defer wg.Done()
	consecutiveFailures := 0
	for {
		latency := pingICMP(ip)
		if latency == 0 {
			if getHealthStatus(ICMPHEALTH, ip) {
				consecutiveFailures++
				if consecutiveFailures > retryBuffer {
					utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("icmp ::::: could not connect to address: (%s)", ip), "error", STDOUT)
					setHealthStatus(ICMPHEALTH, ip, false)
					sendToDiscordWebhook(discordWebhookURL, "Unable to Connect", "ICMP [BROKEN]", 0xFF0000, hostname, "N/A", tags)
				}
			}
		} else {
			if !getHealthStatus(ICMPHEALTH, ip) {
				setHealthStatus(ICMPHEALTH, ip, true)
				sendToDiscordWebhook(discordWebhookURL, "Connection Success", "ICMP SUCCESS", 0x00FF00, hostname, "N/A", tags)
			}
			utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("icmp ::::: %s (%s) (%s) ::::: latency %v", hostname, ip, utils.FormatTags(tags), latency), "info", STDOUT)
			consecutiveFailures = 0
		}
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}
func pingTaskHTTP(fqdn string, tags []string, timeout, retryBuffer int, skipVerify bool, wg *sync.WaitGroup, discordWebhookURL string) {
	defer wg.Done()
	consecutiveFailures := 0
	for {
		respCode, err := pingHTTP(fqdn, skipVerify)
		if err != nil || respCode == 0 {
			if getHealthStatus(HTTPHEALTH, fqdn) {
				consecutiveFailures++
				if consecutiveFailures > retryBuffer {
					utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("http ::::: could not connect to host: (%s)", fqdn), "error", STDOUT)
					setHealthStatus(HTTPHEALTH, fqdn, false)
					sendToDiscordWebhook(discordWebhookURL, "Unable to Connect", "HTTP [BROKEN]", 0xFF0000, "N/A", fqdn, tags)
				}
			}
		} else if respCode == 200 || respCode == 201 || respCode == 204 {
			if !getHealthStatus(HTTPHEALTH, fqdn) {
				setHealthStatus(HTTPHEALTH, fqdn, true)
				sendToDiscordWebhook(discordWebhookURL, "Connection Success", "HTTP SUCCESS", 0x00FF00, "N/A", fqdn, tags)
			}
			utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("http ::::: %s ::: [%v]", fqdn, respCode), "info", STDOUT)
			consecutiveFailures = 0
		}
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}
func pingHTTP(hostname string, skipVerify bool) (int, error) {
	if !strings.HasPrefix(hostname, "http://") && !strings.HasPrefix(hostname, "https://") {
		return 0, nil
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipVerify},
		},
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
func sendToDiscordWebhook(webhookURL, title, description string, color int, hostname, fqdn string, tags []string) {
	if DISCORDDISABLE {
		utils.ConsoleAndLoggerOutput(LOGGER, "syst ::::: discord webhooks disabled", "info", STDOUT)
		return
	}
	embed := utils.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Fields: []utils.DiscordField{
			{Name: "Hostname", Value: hostname, Inline: true},
			{Name: "FQDN", Value: fqdn, Inline: true},
			{Name: "Date", Value: time.Now().Format("2006-01-02"), Inline: true},
			{Name: "Time", Value: time.Now().Format("15:04:05"), Inline: true},
			{Name: "Tags", Value: string(utils.FormatTags(tags)), Inline: true},
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
func healthCheck(timeout int) {
	for {
		for ip := range ICMPHEALTH {
			status := "passing"
			if !getHealthStatus(ICMPHEALTH, ip) {
				status = "failing"
			}
			utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("icmp ::::: health for %s ::::: %s", ip, status), "info", STDOUT)
		}
		for fqdn := range HTTPHEALTH {
			status := "passing"
			if !getHealthStatus(HTTPHEALTH, fqdn) {
				status = "failing"
			}
			utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("http ::::: health for %s ::::: %s", fqdn, status), "info", STDOUT)
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
		setHealthStatus(ICMPHEALTH, icmp.IP, true)
	}
	for _, http := range CONFIG.HTTP {
		setHealthStatus(HTTPHEALTH, http.FQDN, true)
	}
	for _, icmp := range CONFIG.ICMP {
		wg.Add(1)
		go pingTaskICMP(icmp.IP, icmp.Hostname, icmp.Tags, icmp.RetryBuffer, icmp.Timeout, &wg, CONFIG.Configuration.DiscordWebHookURL)
	}
	for _, http := range CONFIG.HTTP {
		wg.Add(1)
		go pingTaskHTTP(http.FQDN, http.Tags, http.Timeout, http.RetryBuffer, http.SkipVerify, &wg, CONFIG.Configuration.DiscordWebHookURL)
	}
	wg.Add(1)
	go healthCheck(HEALTHCHECKTIMEOUT)
	<-signalChannel
	utils.ConsoleAndLoggerOutput(LOGGER, "syst ::::: termination signal received. exiting...", "info", STDOUT)
}
