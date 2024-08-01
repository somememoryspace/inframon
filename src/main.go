package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/somememoryspace/inframon/src/connectors"
	"github.com/somememoryspace/inframon/src/notifiers"
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

func pingTaskICMP(address string, service string, retryBuffer int, timeout int, networkZone string, instanceType string, wg *sync.WaitGroup) {
	defer wg.Done()
	consecutiveFailures := 0
	for {
		latency := connectors.PingICMP(address)
		if latency == 0 {
			utils.ConsoleAndLoggerOutput(LOGGER, "icmp", fmt.Sprintf("connection[KO] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: latency[%v]", address, service, networkZone, instanceType, latency), "error", STDOUT)
			if getHealthStatus(ICMPHEALTH, address) {
				consecutiveFailures++
				if consecutiveFailures > retryBuffer {
					setHealthStatus(ICMPHEALTH, address, false)
					sendNotification(address, service, networkZone, instanceType, "Connection Interrupted", 0xFF0000, latency)
				}
			}
		} else {
			utils.ConsoleAndLoggerOutput(LOGGER, "icmp", fmt.Sprintf("connection[OK] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: latency[%v]", address, service, networkZone, instanceType, latency), "info", STDOUT)
			if !getHealthStatus(ICMPHEALTH, address) {
				setHealthStatus(ICMPHEALTH, address, true)
				sendNotification(address, service, networkZone, instanceType, "Connection Established", 0x00FF00, latency)

			}
			consecutiveFailures = 0
		}
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}

func pingTaskHTTP(address string, service string, retryBuffer int, timeout int, skipVerify bool, networkZone string, instanceType string, wg *sync.WaitGroup) {
	defer wg.Done()
	consecutiveFailures := 0
	for {
		respCode, err := connectors.PingHTTP(address, service, skipVerify)
		if err != nil || respCode == 0 {
			utils.ConsoleAndLoggerOutput(LOGGER, "http", fmt.Sprintf("connection[KO] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: response[%v]", address, service, networkZone, instanceType, respCode), "error", STDOUT)
			if getHealthStatus(HTTPHEALTH, address) {
				consecutiveFailures++
				if consecutiveFailures > retryBuffer {
					setHealthStatus(HTTPHEALTH, address, false)
					sendNotification(address, service, networkZone, instanceType, "Connection Interrupted", 0xFF0000, 0)
				}
			}
		} else if respCode == 200 || respCode == 201 || respCode == 204 {
			utils.ConsoleAndLoggerOutput(LOGGER, "http", fmt.Sprintf("connection[OK] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: response[%v]", address, service, networkZone, instanceType, respCode), "info", STDOUT)
			if !getHealthStatus(HTTPHEALTH, address) {
				setHealthStatus(HTTPHEALTH, address, true)
				sendNotification(address, service, networkZone, instanceType, "Connection Established", 0x00FF00, 0)
			}
			consecutiveFailures = 0
		}
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}

func healthCheck(timeout int) {
	for {
		for address := range ICMPHEALTH {
			status := "PASS"
			if !getHealthStatus(ICMPHEALTH, address) {
				status = "FAIL"
			}
			utils.ConsoleAndLoggerOutput(LOGGER, "icmp", fmt.Sprintf("health[%s] :: address[%s]", status, address), "info", STDOUT)
		}
		for address := range HTTPHEALTH {
			status := "PASS"
			if !getHealthStatus(HTTPHEALTH, address) {
				status = "FAIL"
			}
			utils.ConsoleAndLoggerOutput(LOGGER, "http", fmt.Sprintf("health[%s] :: address[%s]", status, address), "info", STDOUT)
		}
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}

func sendNotification(address, service, networkZone, instanceType, status string, color int, latency time.Duration) {
	message := fmt.Sprintf("Status Change: [%s]", service)

	// Notify Discord
	errDiscord := notifiers.SendToDiscordWebhook(DISCORDDISABLE, CONFIG.Configuration.DiscordWebHookURL, status, message, color, address, service, networkZone, instanceType, 0, 5*time.Second, 5)
	if errDiscord != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("notification[DISCORD] :: unable to send discord webhook notification :: [%s]", errDiscord), "error", STDOUT)
	} else {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", "notification[DISCORD] :: successfully sent discord webhook notification]", "info", STDOUT)
	}

	// Notify SMTP
	errSmtp := notifiers.SendSMTPMail(CONFIG.Configuration.SmtpDisable, CONFIG.Configuration.SmtpUsername, CONFIG.Configuration.SmtpPassword, CONFIG.Configuration.SmtpHost, CONFIG.Configuration.SmtpTo, CONFIG.Configuration.SmtpFrom, CONFIG.Configuration.SmtpPort, status, message)
	if errSmtp != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("notification[SMTP] :: unable to send smtp push notification :: [%s]", errSmtp), "error", STDOUT)
	} else {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", "notification[SMTP] :: successfully sent smtp push notification]", "info", STDOUT)
	}
}

func main() {
	utils.ConsoleAndLoggerOutput(LOGGER, "system", "runtime[MAIN] :: starting service", "info", STDOUT)

	var wg sync.WaitGroup

	for _, icmpConfig := range CONFIG.ICMP {
		setHealthStatus(ICMPHEALTH, icmpConfig.Address, true)
		wg.Add(1)
		go pingTaskICMP(icmpConfig.Address, icmpConfig.Service, icmpConfig.RetryBuffer, icmpConfig.Timeout, icmpConfig.NetworkZone, icmpConfig.InstanceType, &wg)
	}

	for _, httpConfig := range CONFIG.HTTP {
		setHealthStatus(HTTPHEALTH, httpConfig.Address, true)
		wg.Add(1)
		go pingTaskHTTP(httpConfig.Address, httpConfig.Service, httpConfig.RetryBuffer, httpConfig.Timeout, httpConfig.SkipVerify, httpConfig.NetworkZone, httpConfig.InstanceType, &wg)
	}
	wg.Add(1)
	go healthCheck(HEALTHCHECKTIMEOUT)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		utils.ConsoleAndLoggerOutput(LOGGER, "system", "runtime[MAIN] :: shutting down inframon system", "info", STDOUT)
		os.Exit(0)
	}()
	wg.Wait()
}
