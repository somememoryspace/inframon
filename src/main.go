package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/somememoryspace/inframon/src/connectors"
	"github.com/somememoryspace/inframon/src/notifiers"
	"github.com/somememoryspace/inframon/src/utils"
)

var (
	MUTEX              sync.Mutex
	CONFIGARG          = flag.String("config", "", "path/to/file targeting inframon config.yaml file")
	LOGPATHARG         = flag.String("logpath", "", "path/to/logfile targeting inframon log file")
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

	if !CONFIG.Configuration.Stdout {
		if *LOGPATHARG == "" {
			log.Fatal("no logfile path provided")
		}
	}

	LOGGER = utils.SetupLogger(CONFIG.Configuration.Stdout, CONFIG.Configuration.LogFileDirectory, CONFIG.Configuration.LogFileName)

	if err := utils.ValidateICMPConfig(CONFIG.ICMP); err != nil {
		log.Fatalf("invalid icmp configuration: %v", err)
	}

	if err := utils.ValidateHTTPConfig(CONFIG.HTTP); err != nil {
		log.Fatalf("invalid http configuration: %v", err)
	}

	if err := utils.ValidateConfiguration(CONFIG); err != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("configuration validation failed: %v", err), "error")
		log.Fatalf("configuration validation failed: %v", err)
	}

	DISCORDDISABLE = CONFIG.Configuration.DiscordWebHookDisable
	HEALTHCHECKTIMEOUT = CONFIG.Configuration.HealthCheckTimeout

	utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("variable[privilegedMode] :: %v", CONFIG.Configuration.PrivilegedMode), "info")
	utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("variable[stdOut] :: %v", CONFIG.Configuration.Stdout), "info")
	utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("variable[healthCheckTimeout] :: %v", CONFIG.Configuration.HealthCheckTimeout), "info")
	utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("variable[discordWebhookDisable] :: %v", CONFIG.Configuration.DiscordWebHookDisable), "info")
	utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("variable[smtpDisable] :: %v", CONFIG.Configuration.SmtpDisable), "info")
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

func pingTaskICMP(privileged bool, address string, service string, retryBuffer int, timeout int, failureTimeout int, networkZone string, instanceType string, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		latency, err := connectors.PingICMP(address, privileged, retryBuffer, failureTimeout)
		if latency == 0 {
			utils.ConsoleAndLoggerOutput(LOGGER, "icmp", fmt.Sprintf("connection[KO] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: latency[%v] error[%v]", address, service, networkZone, instanceType, latency, err), "error")
			if getHealthStatus(ICMPHEALTH, address) {
				setHealthStatus(ICMPHEALTH, address, false)
				sendNotification("ICMP", address, service, networkZone, instanceType, "Connection Interrupted", 0xFF0000, latency)
			}
		} else {
			utils.ConsoleAndLoggerOutput(LOGGER, "icmp", fmt.Sprintf("connection[OK] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: latency[%v]", address, service, networkZone, instanceType, latency), "info")
			if !getHealthStatus(ICMPHEALTH, address) {
				setHealthStatus(ICMPHEALTH, address, true)
				sendNotification("ICMP", address, service, networkZone, instanceType, "Connection Established", 0x00FF00, latency)
			}
		}
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}

func pingTaskHTTP(address string, service string, retryBuffer int, timeout int, failureTimeout int, skipVerify bool, networkZone string, instanceType string, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		respCode, err := connectors.PingHTTP(address, service, skipVerify, retryBuffer, failureTimeout)
		if err != nil || respCode == 0 {
			utils.ConsoleAndLoggerOutput(LOGGER, "http", fmt.Sprintf("connection[KO] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: response[%d] error[%v]", address, service, networkZone, instanceType, respCode, err), "error")
			if getHealthStatus(HTTPHEALTH, address) {
				setHealthStatus(HTTPHEALTH, address, false)
				sendNotification("HTTP", address, service, networkZone, instanceType, "Connection Interrupted", 0xFF0000, 0)
			}
		} else if respCode == 200 || respCode == 201 || respCode == 204 {
			utils.ConsoleAndLoggerOutput(LOGGER, "http", fmt.Sprintf("connection[OK] :: address[%s] service[%s] networkzone[%s] instancetype[%s] :: response[%d]", address, service, networkZone, instanceType, respCode), "info")
			if !getHealthStatus(HTTPHEALTH, address) {
				setHealthStatus(HTTPHEALTH, address, true)
				sendNotification("HTTP", address, service, networkZone, instanceType, "Connection Established", 0x00FF00, 0)
			}
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
			utils.ConsoleAndLoggerOutput(LOGGER, "icmp", fmt.Sprintf("health[%s] :: address[%s]", status, address), "info")
		}
		for address := range HTTPHEALTH {
			status := "PASS"
			if !getHealthStatus(HTTPHEALTH, address) {
				status = "FAIL"
			}
			utils.ConsoleAndLoggerOutput(LOGGER, "http", fmt.Sprintf("health[%s] :: address[%s]", status, address), "info")
		}
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}

func sendNotification(scanType string, address string, service string, networkZone string, instanceType string, status string, color int, latency time.Duration) {
	message := fmt.Sprintf("Status Change: [%s]", scanType)

	// Notify Discord
	errDiscord := notifiers.SendToDiscordWebhook(DISCORDDISABLE, CONFIG.Configuration.DiscordWebHookURL, status, message, color, address, service, networkZone, instanceType, 0, 5*time.Second, 5)
	if errDiscord != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("notification[DISCORD] :: unable to send discord webhook notification :: [%s]", errDiscord), "error")
	} else {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", "notification[DISCORD] :: successfully sent discord webhook notification]", "info")
	}

	// Notify SMTP
	errSmtp := notifiers.SendSMTPMail(CONFIG.Configuration.SmtpDisable, CONFIG.Configuration.SmtpUsername, CONFIG.Configuration.SmtpPassword, CONFIG.Configuration.SmtpHost, CONFIG.Configuration.SmtpTo, CONFIG.Configuration.SmtpFrom, CONFIG.Configuration.SmtpPort, status, message, address, service, networkZone, instanceType)
	if errSmtp != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", fmt.Sprintf("notification[SMTP] :: unable to send smtp push notification :: [%s]", errSmtp), "error")
	} else {
		utils.ConsoleAndLoggerOutput(LOGGER, "system", "notification[SMTP] :: successfully sent smtp push notification]", "info")
	}
}

func isRunningAsRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		fmt.Printf("error getting current user: %v\n", err)
		return false
	}
	uid, err := strconv.Atoi(currentUser.Uid)
	if err != nil {
		fmt.Printf("error converting UID to integer: %v\n", err)
		return false
	}
	return uid == 0
}

func checkPrivileges(privilegedMode bool, logger *log.Logger) {
	if privilegedMode != isRunningAsRoot() {
		mode := "privileged"
		userStatus := "unprivileged user"
		if !privilegedMode {
			mode = "unprivileged"
			userStatus = "privileged user"
		}
		message := fmt.Sprintf("runtime[MAIN] :: running %s mode with %s", mode, userStatus)
		utils.ConsoleAndLoggerOutput(logger, "system", message, "error")
		panic(fmt.Sprintf("error[MAIN] :: running %s mode with %s", mode, userStatus))
	}
}

func main() {
	utils.ConsoleAndLoggerOutput(LOGGER, "system", "runtime[MAIN] :: starting service", "info")
	checkPrivileges(CONFIG.Configuration.PrivilegedMode, LOGGER)

	var wg sync.WaitGroup

	for _, icmpConfig := range CONFIG.ICMP {
		setHealthStatus(ICMPHEALTH, icmpConfig.Address, true)
		wg.Add(1)
		go pingTaskICMP(CONFIG.Configuration.PrivilegedMode, icmpConfig.Address, icmpConfig.Service, icmpConfig.RetryBuffer, icmpConfig.Timeout, icmpConfig.FailureTimeout, icmpConfig.NetworkZone, icmpConfig.InstanceType, &wg)
	}

	for _, httpConfig := range CONFIG.HTTP {
		setHealthStatus(HTTPHEALTH, httpConfig.Address, true)
		wg.Add(1)
		go pingTaskHTTP(httpConfig.Address, httpConfig.Service, httpConfig.RetryBuffer, httpConfig.Timeout, httpConfig.FailureTimeout, httpConfig.SkipVerify, httpConfig.NetworkZone, httpConfig.InstanceType, &wg)
	}
	wg.Add(1)
	go healthCheck(HEALTHCHECKTIMEOUT)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		utils.ConsoleAndLoggerOutput(LOGGER, "system", "runtime[MAIN] :: shutting down inframon system", "info")
		os.Exit(0)
	}()
	wg.Wait()
}
