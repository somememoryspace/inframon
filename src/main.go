package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
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
	CONFIGARG          = flag.String("config_path", "", "path/to/file targeting inframon config.yaml file")
	CONFIG             *utils.Config
	LOGGER             *log.Logger
	ICMPHEALTH         = make(map[string]bool)
	HTTPHEALTH         = make(map[string]bool)
	DISCORDDISABLE     bool
	HEALTHCHECKTIMEOUT int
	STDOUT             bool
)

func init() {
	flag.Parse()
	if *CONFIGARG == "" {
		log.Fatal("no configuration path provided")
	}

	CONFIG = utils.ParseConfig(*CONFIGARG)
	LOGGER = utils.SetupLogger(CONFIG.Configuration.LogFileDirectory, CONFIG.Configuration.LogFileName)

	if err := utils.ValidateICMPConfig(CONFIG.ICMP); err != nil {
		log.Fatalf("invalid icmp configuration: %v", err)
	}

	if err := utils.ValidateHTTPConfig(CONFIG.HTTP); err != nil {
		log.Fatalf("invalid http configuration: %v", err)
	}

	if err := utils.ValidateConfiguration(CONFIG); err != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("configuration validation failed: %v", err), "error", STDOUT)
		log.Fatalf("configuration validation failed: %v", err)
	}

	DISCORDDISABLE = CONFIG.Configuration.DiscordWebHookDisable
	HEALTHCHECKTIMEOUT = CONFIG.Configuration.HealthCheckTimeout
	STDOUT = CONFIG.Configuration.Stdout
}

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

func pingTaskICMP(address string, service string, retryBuffer int, timeout int, networkZone string, instanceType string, wg *sync.WaitGroup, discordWebhookURL string) {
	defer wg.Done()
	consecutiveFailures := 0
	for {
		latency := pingICMP(address)
		if latency == 0 {
			if getHealthStatus(ICMPHEALTH, address) {
				consecutiveFailures++
				if consecutiveFailures > retryBuffer {
					utils.ConsoleAndLoggerOutput(LOGGER, "  icmp", fmt.Sprintf("connection[KO] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: latency[%v]", address, service, networkZone, instanceType, latency), "error", STDOUT)
					setHealthStatus(ICMPHEALTH, address, false)
					sendToDiscordWebhook(discordWebhookURL, "Connection Interrupted", "ICMP :: KO", 0xFF0000, address, service, networkZone, instanceType)
					sendSMTPMail(CONFIG, "Connection Interrupted", fmt.Sprintf("ICMP :: KO :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: latency[%v]", address, service, networkZone, instanceType, latency))
				}
			}
		} else {
			if !getHealthStatus(ICMPHEALTH, address) {
				setHealthStatus(ICMPHEALTH, address, true)
				sendToDiscordWebhook(discordWebhookURL, "Connection Established", "ICMP :: OK", 0x00FF00, address, service, networkZone, instanceType)
				sendSMTPMail(CONFIG, "Connection Established", fmt.Sprintf("ICMP :: OK :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: latency[%v]", address, service, networkZone, instanceType, latency))
			}
			utils.ConsoleAndLoggerOutput(LOGGER, "  icmp", fmt.Sprintf("connection[OK] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: latency[%v]", address, service, networkZone, instanceType, latency), "info", STDOUT)
			consecutiveFailures = 0
		}
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}

func pingTaskHTTP(address string, service string, retryBuffer int, timeout int, skipVerify bool, networkZone string, instanceType string, wg *sync.WaitGroup, discordWebhookURL string) {
	defer wg.Done()
	consecutiveFailures := 0
	for {
		respCode, err := pingHTTP(address, service, skipVerify)
		if err != nil || respCode == 0 {
			if getHealthStatus(HTTPHEALTH, address) {
				consecutiveFailures++
				if consecutiveFailures > retryBuffer {
					utils.ConsoleAndLoggerOutput(LOGGER, "  http", fmt.Sprintf("connection[KO] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: response[%v]", address, service, networkZone, instanceType, respCode), "error", STDOUT)
					setHealthStatus(HTTPHEALTH, address, false)
					sendToDiscordWebhook(discordWebhookURL, "Connection Interrupted", "HTTP :: KO", 0xFF0000, address, service, networkZone, instanceType)
					sendSMTPMail(CONFIG, "Connection Interrupted", fmt.Sprintf("HTTP :: KO :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: response[%v]", address, service, networkZone, instanceType, respCode))
				}
			}
		} else if respCode == 200 || respCode == 201 || respCode == 204 {
			if !getHealthStatus(HTTPHEALTH, address) {
				setHealthStatus(HTTPHEALTH, address, true)
				sendToDiscordWebhook(discordWebhookURL, "Connection Established", "HTTP :: OK", 0x00FF00, address, service, networkZone, instanceType)
				sendSMTPMail(CONFIG, "Connection Established", fmt.Sprintf("HTTP :: OK :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: response[%v]", address, service, networkZone, instanceType, respCode))
			}
			utils.ConsoleAndLoggerOutput(LOGGER, "  http", fmt.Sprintf("connection[OK] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: response[%v]", address, service, networkZone, instanceType, respCode), "info", STDOUT)
			consecutiveFailures = 0
		}
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}

func pingHTTP(address string, service string, skipVerify bool) (int, error) {
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

func sendToDiscordWebhook(webhookURL, title, description string, color int, address string, service string, networkZone string, instanceType string) {
	if DISCORDDISABLE {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", "discord webhook notifications disabled", "info", STDOUT)
		return
	}
	embed := utils.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Fields: []utils.DiscordField{
			{Name: "Address", Value: address, Inline: true},
			{Name: "Service", Value: service, Inline: true},
			{Name: "Date", Value: time.Now().Format("2006-01-02"), Inline: true},
			{Name: "Time", Value: time.Now().Format("15:04:05"), Inline: true},
			{Name: "NetworkZone", Value: networkZone, Inline: true},
			{Name: "InstanceType", Value: instanceType, Inline: true},
		},
	}
	message := utils.Message{
		Embeds: []utils.DiscordEmbed{embed},
	}
	jsonPayload, err := json.Marshal(message)
	if err != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("could not marshal json :: error[%v]", err), "error", STDOUT)
		return
	}
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("could not send discord webhook :: error[%v]", err), "error", STDOUT)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("unexpected :: code[%d]", resp.StatusCode), "error", STDOUT)
		return
	}
	utils.ConsoleAndLoggerOutput(LOGGER, "system", "message sent successfully to discord webhook", "info", STDOUT)
}

func sendSMTPMail(config *utils.Config, subject, body string) error {
	if CONFIG.Configuration.SmtpDisable {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", "smtp notifications are disabled", "info", STDOUT)
		return nil
	}
	auth := smtp.PlainAuth("", config.Configuration.SmtpUsername, config.Configuration.SmtpPassword, config.Configuration.SmtpHost)
	to := []string{config.Configuration.SmtpTo}
	msg := []byte("From: " + config.Configuration.SmtpFrom + "\r\n" +
		"To: " + config.Configuration.SmtpTo + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n")
	addr := fmt.Sprintf("%s:%s", config.Configuration.SmtpHost, config.Configuration.SmtpPort)

	utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("sending smtp email to [%s] via [%s]", strings.Join(to, ", "), addr), "info", STDOUT)
	utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("smtp email subject: [%s]", subject), "info", STDOUT)
	utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("smtp email body: [%s]", body), "info", STDOUT)

	err := smtp.SendMail(addr, auth, config.Configuration.SmtpFrom, to, msg)
	if err != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("error sending email: [%v]", err), "error", STDOUT)
		return fmt.Errorf("could not send smtp email: [%w]", err)
	}
	utils.ConsoleAndLoggerOutput(LOGGER, "system", "email sent successfully", "info", STDOUT)
	return nil
}

func healthCheck(timeout int) {
	for {
		for address := range ICMPHEALTH {
			status := "PASS"
			if !getHealthStatus(ICMPHEALTH, address) {
				status = "FAIL"
			}
			utils.ConsoleAndLoggerOutput(LOGGER, "  icmp", fmt.Sprintf("health[%s] :: address[%s]", status, address), "info", STDOUT)
		}
		for address := range HTTPHEALTH {
			status := "PASS"
			if !getHealthStatus(HTTPHEALTH, address) {
				status = "FAIL"
			}
			utils.ConsoleAndLoggerOutput(LOGGER, "  http", fmt.Sprintf("health[%s] :: address[%s]", status, address), "info", STDOUT)
		}
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}

func main() {
	utils.ConsoleAndLoggerOutput(LOGGER, "system", "runtime start", "info", STDOUT)
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	var wg sync.WaitGroup
	for _, icmp := range CONFIG.ICMP {
		setHealthStatus(ICMPHEALTH, icmp.Address, true)
	}
	for _, http := range CONFIG.HTTP {
		setHealthStatus(HTTPHEALTH, http.Address, true)
	}
	for _, icmp := range CONFIG.ICMP {
		wg.Add(1)
		go pingTaskICMP(icmp.Address, icmp.Service, icmp.RetryBuffer, icmp.Timeout, icmp.NetworkZone, icmp.InstanceType, &wg, CONFIG.Configuration.DiscordWebHookURL)
	}
	for _, http := range CONFIG.HTTP {
		wg.Add(1)
		go pingTaskHTTP(http.Address, http.Service, http.RetryBuffer, http.Timeout, http.SkipVerify, http.NetworkZone, http.InstanceType, &wg, CONFIG.Configuration.DiscordWebHookURL)
	}
	wg.Add(1)
	go healthCheck(HEALTHCHECKTIMEOUT)
	<-signalChannel
	utils.ConsoleAndLoggerOutput(LOGGER, "system", "termination signal received :: exiting...", "info", STDOUT)
}
