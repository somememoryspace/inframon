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
	CONFIGARG          = flag.String("config", "", "path/to/file targeting inframon config.yaml file")
	LOGPATHARG         = flag.String("logpath", "", "path/to/logfile targeting inframon log file")
	LOGNAMEARG         = flag.String("logname", "", "file name for the log file")
	CONFIG             *utils.Config
	LOGGER             *utils.SafeLogger
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
	utils.CheckPrivileges(CONFIG.Configuration.PrivilegedMode)

	var err error
	LOGGER, err = utils.SetupLogger(CONFIG.Configuration.Stdout, *LOGPATHARG, *LOGNAMEARG)
	if err != nil || LOGGER == nil {
		log.Fatalf("could not setup logger: %v", err)
	}

	if !CONFIG.Configuration.Stdout {
		if *LOGPATHARG == "" {
			log.Fatal("no logpath path provided in inframon startup argument")
		}
		if *LOGNAMEARG == "" {
			log.Fatal("no logname path provided in inframon startup argument")
		}
		if CONFIG.Configuration.LogFileSize == "" {
			log.Fatal("no logFileSize value provided in config file")
		}
		if CONFIG.Configuration.MaxLogFileKeep <= 0 {
			log.Fatal("maxLogFileKeep value provided in config file must be greather than 0")
		}
	}

	if err := utils.ValidateICMPConfig(CONFIG.ICMP); err != nil {
		log.Fatalf("invalid icmp configuration: %v", err)
	}

	if err := utils.ValidateHTTPConfig(CONFIG.HTTP); err != nil {
		log.Fatalf("invalid http configuration: %v", err)
	}

	if err := utils.ValidateConfiguration(CONFIG); err != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, "STARTUP", fmt.Sprintf("Configuration validation failed: %v", err), "ERROR")
		log.Fatalf("configuration validation failed: %v", err)
	}

	DISCORDDISABLE = CONFIG.Configuration.DiscordWebHookDisable
	HEALTHCHECKTIMEOUT = CONFIG.Configuration.HealthCheckTimeout

	sendNotificationSystem("Booting Inframon Service", "Inframon Started")
	utils.ConsoleAndLoggerOutput(LOGGER, "STARTUP", fmt.Sprintf("privilegedMode :: [%v]", CONFIG.Configuration.PrivilegedMode), "INFO")
	utils.ConsoleAndLoggerOutput(LOGGER, "STARTUP", fmt.Sprintf("stdOut :: [%v]", CONFIG.Configuration.Stdout), "INFO")
	utils.ConsoleAndLoggerOutput(LOGGER, "STARTUP", fmt.Sprintf("healthCheckTimeout :: [%v]", CONFIG.Configuration.HealthCheckTimeout), "INFO")
	utils.ConsoleAndLoggerOutput(LOGGER, "STARTUP", fmt.Sprintf("discordWebhookDisable :: [%v]", CONFIG.Configuration.DiscordWebHookDisable), "INFO")
	utils.ConsoleAndLoggerOutput(LOGGER, "STARTUP", fmt.Sprintf("smtpDisable :: [%v]", CONFIG.Configuration.SmtpDisable), "INFO")
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
			utils.ConsoleAndLoggerOutput(LOGGER, "ICMP KO", fmt.Sprintf("Address: [%s] Service: [%s] NetworkZone: [%s] InstanceType: [%s] Latency: [%v] Error: [%v]", address, service, networkZone, instanceType, latency, err), "ERROR")
			if getHealthStatus(ICMPHEALTH, address) {
				setHealthStatus(ICMPHEALTH, address, false)
				sendNotification("Status Changed :: ICMP", address, service, networkZone, instanceType, "Connection Interrupted", 0xFF0000, latency)
			}
		} else {
			utils.ConsoleAndLoggerOutput(LOGGER, "ICMP OK", fmt.Sprintf("Address: [%s] Service: [%s] NetworkZone: [%s] InstanceType: [%s] Latency: [%v]", address, service, networkZone, instanceType, latency), "INFO")
			if !getHealthStatus(ICMPHEALTH, address) {
				setHealthStatus(ICMPHEALTH, address, true)
				sendNotification("Status Changed :: ICMP", address, service, networkZone, instanceType, "Connection Established", 0x00FF00, latency)
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
			utils.ConsoleAndLoggerOutput(LOGGER, "HTTP KO", fmt.Sprintf("Address: [%s] Service: [%s] NetworkZone: [%s] InstanceType: [%s] Response: [%d] Error: [%v]", address, service, networkZone, instanceType, respCode, err), "ERROR")
			if getHealthStatus(HTTPHEALTH, address) {
				setHealthStatus(HTTPHEALTH, address, false)
				sendNotification("Status Changed :: HTTP", address, service, networkZone, instanceType, "Connection Interrupted", 0xFF0000, 0)
			}
		} else if respCode == 200 || respCode == 201 || respCode == 204 {
			utils.ConsoleAndLoggerOutput(LOGGER, "HTTP OK", fmt.Sprintf("Address: [%s] Service: [%s] NetworkZone: [%s] InstanceType: [%s] Response: [%d]", address, service, networkZone, instanceType, respCode), "INFO")
			if !getHealthStatus(HTTPHEALTH, address) {
				setHealthStatus(HTTPHEALTH, address, true)
				sendNotification("Status Changed :: HTTP", address, service, networkZone, instanceType, "Connection Established", 0xFF0000, 0)
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
			utils.ConsoleAndLoggerOutput(LOGGER, "ICMP HEALTH", fmt.Sprintf("Health [%s] Address [%s]", status, address), "INFO")
		}
		for address := range HTTPHEALTH {
			status := "PASS"
			if !getHealthStatus(HTTPHEALTH, address) {
				status = "FAIL"
			}
			utils.ConsoleAndLoggerOutput(LOGGER, "HTTP HEALTH", fmt.Sprintf("Health [%s] Address [%s]", status, address), "INFO")
		}
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}

func sendNotification(message string, address string, service string, networkZone string, instanceType string, status string, color int, latency time.Duration) {

	var errDiscord error
	var errSmtp error

	errDiscord = notifiers.SendToDiscordWebhook(DISCORDDISABLE, CONFIG.Configuration.DiscordWebHookURL, status, message, color, address, service, networkZone, instanceType, 0, 5*time.Second, 5)
	errSmtp = notifiers.SendSMTPMail(
		CONFIG.Configuration.SmtpDisable,
		CONFIG.Configuration.SmtpUsername,
		CONFIG.Configuration.SmtpPassword,
		CONFIG.Configuration.SmtpHost,
		CONFIG.Configuration.SmtpTo,
		CONFIG.Configuration.SmtpFrom,
		CONFIG.Configuration.SmtpPort,
		status, message, address, service, networkZone, instanceType,
	)

	if errDiscord != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, "DISCORD NOTIFICATION", fmt.Sprintf("Unable to send discord webhook notification :: [%s]", errDiscord), "ERROR")
	} else {
		utils.ConsoleAndLoggerOutput(LOGGER, "DISCORD NOTIFICATION", "Successfully sent discord webhook notification", "INFO")
	}

	if errSmtp != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, "SMTP NOTIFICATION", fmt.Sprintf("Unable to send smtp push notification :: [%s]", errSmtp), "ERROR")
	} else {
		utils.ConsoleAndLoggerOutput(LOGGER, "SMTP NOTIFICATION", "Successfully sent smtp push notification", "INFO")
	}
}

func sendNotificationSystem(message string, status string) {
	errDiscord := notifiers.SendToDiscordWebhookSystem(DISCORDDISABLE, CONFIG.Configuration.DiscordWebHookURL, status, message, 0x4682B4, 0, 5*time.Second, 5)
	if errDiscord != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, "DISCORD NOTIFICATION", fmt.Sprintf("Unable to send discord webhook notification :: [%s]", errDiscord), "ERROR")
	} else {
		utils.ConsoleAndLoggerOutput(LOGGER, "DISCORD NOTIFICATION", "Successfully sent discord webhook notification", "INFO")
	}
}

func main() {
	utils.ConsoleAndLoggerOutput(LOGGER, "STARTUP", "Starting Inframon", "INFO")

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

	if !CONFIG.Configuration.Stdout {
		logFileSize := CONFIG.Configuration.LogFileSize
		logFileSizeConverted, err := utils.ConvertToBytes(logFileSize)
		if err != nil {
			panic(fmt.Sprintf("Error rotating logfile. Could not convert input logFileSize in config file: %v", err))
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				time.Sleep(750 * time.Millisecond)
				err := LOGGER.RotateLogFile(*LOGPATHARG, *LOGNAMEARG, logFileSizeConverted, CONFIG.Configuration.MaxLogFileKeep)
				if err != nil {
					utils.ConsoleAndLoggerOutput(LOGGER, "LOG ROTATE", fmt.Sprintf("Error rotating log file: %v", err), "ERROR")
				}
			}
		}()
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	sendNotificationSystem("Shutting Down Inframon", "Inframon Shutting Down")
	utils.ConsoleAndLoggerOutput(LOGGER, "EXIT", "Shutting down inframon system", "INFO")
	go func() {
		<-signalChan
		os.Exit(0)
	}()
	wg.Wait()
}
