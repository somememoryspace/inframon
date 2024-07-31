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
			utils.ConsoleAndLoggerOutput(LOGGER, "  icmp", fmt.Sprintf("connection[KO] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: latency[%v]", address, service, networkZone, instanceType, latency), "error", STDOUT)
			if getHealthStatus(ICMPHEALTH, address) {
				consecutiveFailures++
				if consecutiveFailures > retryBuffer {
					setHealthStatus(ICMPHEALTH, address, false)
					sendToDiscordWebhook(discordWebhookURL, "Connection Interrupted", "ICMP :: KO", 0xFF0000, address, service, networkZone, instanceType, 0, 5*time.Second, 5)
					sendSMTPMail(CONFIG, "Connection Interrupted", fmt.Sprintf("ICMP :: KO :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: latency[%v]", address, service, networkZone, instanceType, latency))
				}
			}
		} else {
			utils.ConsoleAndLoggerOutput(LOGGER, "  icmp", fmt.Sprintf("connection[OK] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: latency[%v]", address, service, networkZone, instanceType, latency), "info", STDOUT)
			if !getHealthStatus(ICMPHEALTH, address) {
				setHealthStatus(ICMPHEALTH, address, true)
				sendToDiscordWebhook(discordWebhookURL, "Connection Established", "ICMP :: OK", 0x00FF00, address, service, networkZone, instanceType, 0, 5*time.Second, 5)
				sendSMTPMail(CONFIG, "Connection Established", fmt.Sprintf("ICMP :: OK :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: latency[%v]", address, service, networkZone, instanceType, latency))
			}
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
			utils.ConsoleAndLoggerOutput(LOGGER, "  http", fmt.Sprintf("connection[KO] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: response[%v]", address, service, networkZone, instanceType, respCode), "error", STDOUT)
			if getHealthStatus(HTTPHEALTH, address) {
				consecutiveFailures++
				if consecutiveFailures > retryBuffer {
					setHealthStatus(HTTPHEALTH, address, false)
					sendToDiscordWebhook(discordWebhookURL, "Connection Interrupted", "HTTP :: KO", 0xFF0000, address, service, networkZone, instanceType, 0, 5*time.Second, 5)
					sendSMTPMail(CONFIG, "Connection Interrupted", fmt.Sprintf("HTTP :: KO :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: response[%v]", address, service, networkZone, instanceType, respCode))
				}
			}
		} else if respCode == 200 || respCode == 201 || respCode == 204 {
			utils.ConsoleAndLoggerOutput(LOGGER, "  http", fmt.Sprintf("connection[OK] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: response[%v]", address, service, networkZone, instanceType, respCode), "info", STDOUT)
			if !getHealthStatus(HTTPHEALTH, address) {
				setHealthStatus(HTTPHEALTH, address, true)
				sendToDiscordWebhook(discordWebhookURL, "Connection Established", "HTTP :: OK", 0x00FF00, address, service, networkZone, instanceType, 0, 5*time.Second, 5)
				sendSMTPMail(CONFIG, "Connection Established", fmt.Sprintf("HTTP :: OK :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: response[%v]", address, service, networkZone, instanceType, respCode))
			}
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

func sendToDiscordWebhook(webhookURL, title, description string, color int, address string, service string, networkZone string, instanceType string, retryCount int, rateLimitResetTime time.Duration, maxRetries int) {
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
	payload, err := json.Marshal(message)
	if err != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("failed to marshal discord webhook payload: %v", err), "error", STDOUT)
		return
	}

	for i := 0; i <= maxRetries; i++ {
		req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(payload))
		if err != nil {
			utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("failed to create discord webhook request: %v", err), "error", STDOUT)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("failed to send discord webhook request: %v", err), "error", STDOUT)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusTooManyRequests {
				if retryCount < maxRetries {
					retryCount++
					utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("rate limited by Discord, retrying in %v seconds... [Attempt %d/%d]", rateLimitResetTime.Seconds(), retryCount, maxRetries), "info", STDOUT)
					time.Sleep(rateLimitResetTime)
					continue
				} else {
					utils.ConsoleAndLoggerOutput(LOGGER, "system", "max retries reached, aborting...", "error", STDOUT)
				}
			} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
				utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("discord webhook returned non-success status: [%v]", resp.Status), "error", STDOUT)
			} else {
				utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("discord webhook notification sent successfully: [%v]", resp.StatusCode), "info", STDOUT)
			}
		}
		break
	}
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
	utils.ConsoleAndLoggerOutput(LOGGER, "system", "starting inframon system", "info", STDOUT)

	var wg sync.WaitGroup

	for _, icmpConfig := range CONFIG.ICMP {
		setHealthStatus(ICMPHEALTH, icmpConfig.Address, true)
		wg.Add(1)
		go pingTaskICMP(icmpConfig.Address, icmpConfig.Service, icmpConfig.RetryBuffer, icmpConfig.Timeout, icmpConfig.NetworkZone, icmpConfig.InstanceType, &wg, CONFIG.Configuration.DiscordWebHookURL)
	}

	for _, httpConfig := range CONFIG.HTTP {
		setHealthStatus(HTTPHEALTH, httpConfig.Address, true)
		wg.Add(1)
		go pingTaskHTTP(httpConfig.Address, httpConfig.Service, httpConfig.RetryBuffer, httpConfig.Timeout, httpConfig.SkipVerify, httpConfig.NetworkZone, httpConfig.InstanceType, &wg, CONFIG.Configuration.DiscordWebHookURL)
	}
	wg.Add(1)
	go healthCheck(HEALTHCHECKTIMEOUT)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		utils.ConsoleAndLoggerOutput(LOGGER, "system", "shutting down inframon system", "info", STDOUT)
		os.Exit(0)
	}()

	wg.Wait()
}
