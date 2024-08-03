package utils

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"net/mail"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ICMP []struct {
		Address        string `yaml:"address"`
		Service        string `yaml:"service"`
		Timeout        int    `yaml:"timeout"`
		FailureTimeout int    `yaml:"failureTimeout"`
		RetryBuffer    int    `yaml:"retryBuffer"`
		NetworkZone    string `yaml:"networkZone"`
		InstanceType   string `yaml:"instanceType"`
	} `yaml:"icmp"`

	HTTP []struct {
		Address        string `yaml:"address"`
		Service        string `yaml:"service"`
		Timeout        int    `yaml:"timeout"`
		FailureTimeout int    `yaml:"failureTimeout"`
		SkipVerify     bool   `yaml:"skipVerify"`
		RetryBuffer    int    `yaml:"retryBuffer"`
		NetworkZone    string `yaml:"networkZone"`
		InstanceType   string `yaml:"instanceType"`
	} `yaml:"http"`

	Configuration struct {
		PrivilegedMode        bool   `yaml:"privilegedMode"`
		LogFileDirectory      string `yaml:"logFileDirectory"`
		LogFileName           string `yaml:"logFileName"`
		Stdout                bool   `yaml:"stdOut"`
		HealthCheckTimeout    int    `yaml:"healthCheckTimeout"`
		DiscordWebHookDisable bool   `yaml:"discordWebhookDisable"`
		DiscordWebHookURL     string `yaml:"discordWebhookUrl"`
		SmtpDisable           bool   `yaml:"smtpDisable"`
		SmtpHost              string `yaml:"smtpHost"`
		SmtpPort              string `yaml:"smtpPort"`
		SmtpUsername          string `yaml:"smtpUsername"`
		SmtpPassword          string `yaml:"smtpPassword"`
		SmtpFrom              string `yaml:"smtpFrom"`
		SmtpTo                string `yaml:"smtpTo"`
		LXCMode               bool   `yaml:"lxcMode"`
	} `yaml:"configuration"`
}

type Taskette struct {
	Name      string
	Address   string
	Protocol  string
	Status    bool
	Tags      []string
	Statistic string
}

type LogEntry struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Event   string `json:"event"`
}

func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	config := &Config{}
	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func ParseConfig(pathToConfig string) *Config {
	config, err := LoadConfig(pathToConfig)
	if err != nil {
		panic(err)
	}
	return config
}

func CreateLogEntry(Type string, Message string, Event string) (string, error) {
	logEntry := LogEntry{
		Type:    Type,
		Message: Message,
		Event:   Event,
	}
	jsonData, err := json.Marshal(logEntry)
	if err != nil {
		return "", fmt.Errorf("unable to serialize to json: %v", err)
	}
	return string(jsonData), nil
}

func ValidateLogDirectory(directoryPath string) {
	fmt.Printf("message: system :: runtime[LOG] :: validating %s directory available... skipping if available\n", directoryPath)
	err := os.MkdirAll(directoryPath, os.ModePerm)
	if err != nil {
		panic(fmt.Sprintf("message: system :: runtime[LOG] :: unable to create directory: %s", directoryPath))
	}
	fmt.Printf("message: system :: runtime[LOG] :: validated %s directory available\n", directoryPath)
}

func SetupLogger(directoryPath string, logName string) *log.Logger {
	ValidateLogDirectory(directoryPath)
	logFilePath := fmt.Sprintf("%s/runtime.log", directoryPath)
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(fmt.Sprintf("message: system :: runtime[LOG] :: unable to open log file: %s", logFilePath))
	}
	logger := log.New(file, "event: ", log.Ldate|log.Ltime|log.Lshortfile)
	return logger
}

func ConsoleAndLoggerOutput(logger *log.Logger, Type string, Message string, Event string, ConsoleOut bool) {
	currentTime := time.Now()
	formattedTime := currentTime.Format("2006-01-02 15:04:05")
	logEntry, err := CreateLogEntry(Type, Message, Event)
	if err != nil {
		fmt.Printf("system :: runtime[LOG] :: error creating log entry: %v\n", err)
		return
	}
	logger.Println(logEntry)
	if ConsoleOut {
		fmt.Printf("%s ::::: message: %s :: %s ::::: event: %s \n", formattedTime, Type, Message, Event)
	}
}

func ValidateICMPConfig(icmpConfig []struct {
	Address        string `yaml:"address"`
	Service        string `yaml:"service"`
	Timeout        int    `yaml:"timeout"`
	FailureTimeout int    `yaml:"failureTimeout"`
	RetryBuffer    int    `yaml:"retryBuffer"`
	NetworkZone    string `yaml:"networkZone"`
	InstanceType   string `yaml:"instanceType"`
}) error {
	addresses := make(map[string]bool)
	for i, icmp := range icmpConfig {
		if icmp.Address == "" {
			return fmt.Errorf("icmp config at index %d has empty address", i)
		}
		if icmp.Service == "" {
			return fmt.Errorf("icmp config at index %d has empty service", i)
		}
		if icmp.Timeout <= 0 {
			return fmt.Errorf("icmp config at index %d has invalid timeout value", i)
		}
		if icmp.FailureTimeout <= 0 {
			return fmt.Errorf("icmp config at index %d has invalid failureTimeout value", i)
		}
		if icmp.RetryBuffer < 0 {
			return fmt.Errorf("icmp config at index %d has invalid retrybuffer value", i)
		}
		if icmp.NetworkZone == "" {
			return fmt.Errorf("icmp config at index %d has empty networkZone", i)
		}
		if icmp.InstanceType == "" {
			return fmt.Errorf("icmp config at index %d has empty instanceType", i)
		}
		if _, exists := addresses[icmp.Address]; exists {
			return fmt.Errorf("icmp config at index %d has duplicate address: %s", i, icmp.Address)
		}
		addresses[icmp.Address] = true
	}
	return nil
}

func ValidateHTTPConfig(httpConfig []struct {
	Address        string `yaml:"address"`
	Service        string `yaml:"service"`
	Timeout        int    `yaml:"timeout"`
	FailureTimeout int    `yaml:"failureTimeout"`
	SkipVerify     bool   `yaml:"skipVerify"`
	RetryBuffer    int    `yaml:"retryBuffer"`
	NetworkZone    string `yaml:"networkZone"`
	InstanceType   string `yaml:"instanceType"`
}) error {
	addresses := make(map[string]bool)
	for i, http := range httpConfig {
		if http.Address == "" {
			return fmt.Errorf("http config at index %d has empty address", i)
		}
		if http.Service == "" {
			return fmt.Errorf("http config at index %d has empty service", i)
		}
		if http.Timeout <= 0 {
			return fmt.Errorf("http config at index %d has invalid timeout value", i)
		}
		if http.FailureTimeout <= 0 {
			return fmt.Errorf("http config at index %d has invalid failureTimeout value", i)
		}
		if http.RetryBuffer < 0 {
			return fmt.Errorf("http config at index %d has invalid retrybuffer value", i)
		}
		if http.NetworkZone == "" {
			return fmt.Errorf("http config at index %d has empty networkZone", i)
		}
		if http.InstanceType == "" {
			return fmt.Errorf("http config at index %d has empty instanceType", i)
		}
		if _, exists := addresses[http.Address]; exists {
			return fmt.Errorf("http config at index %d has duplicate address: %s", i, http.Address)
		}
		addresses[http.Address] = true
	}
	return nil
}

func ValidateConfiguration(config *Config) error {
	if !config.Configuration.DiscordWebHookDisable && config.Configuration.DiscordWebHookURL == "" {
		return fmt.Errorf("discordWebhookUrl cannot be empty when discordWebhookDisable is false")
	}

	if !config.Configuration.SmtpDisable {
		if config.Configuration.SmtpFrom == "" {
			return fmt.Errorf("smtpFrom cannot be empty when smtpDisable is false")
		}
		if err := validateEmail(config.Configuration.SmtpFrom); err != nil {
			return fmt.Errorf("smtpFrom is invalid: %v", err)
		}
		if config.Configuration.SmtpTo == "" {
			return fmt.Errorf("smtpTo cannot be empty when smtpDisable is false")
		}
		if err := validateEmail(config.Configuration.SmtpTo); err != nil {
			return fmt.Errorf("smtpTo is invalid: %v", err)
		}
		if config.Configuration.SmtpHost == "" {
			return fmt.Errorf("smtpHost cannot be empty when smtpDisable is false")
		}
		if config.Configuration.SmtpPort == "" {
			return fmt.Errorf("smtpPort cannot be empty when smtpDisable is false")
		}
		if err := validatePort(config.Configuration.SmtpPort); err != nil {
			return fmt.Errorf("smtpPort is invalid: %v", err)
		}
		if config.Configuration.SmtpUsername == "" {
			return fmt.Errorf("smtpUsername cannot be empty when smtpDisable is false")
		}
		if config.Configuration.SmtpPassword == "" {
			return fmt.Errorf("smtpPassword cannot be empty when smtpDisable is false")
		}
	}

	if config.Configuration.LogFileDirectory == "" {
		return fmt.Errorf("logFileDirectory cannot be empty")
	}
	if config.Configuration.LogFileName == "" {
		return fmt.Errorf("logFileName cannot be empty")
	}
	if !config.Configuration.Stdout && config.Configuration.LogFileDirectory == "" {
		return fmt.Errorf("either logFileDirectory should be specified or stdout should be true")
	}

	if config.Configuration.HealthCheckTimeout <= 0 {
		return fmt.Errorf("healthCheckTimeout must be greater than 0")
	}

	if err := ValidateICMPConfig(config.ICMP); err != nil {
		return fmt.Errorf("icmp config validation failed: %v", err)
	}

	if err := ValidateHTTPConfig(config.HTTP); err != nil {
		return fmt.Errorf("http config validation failed: %v", err)
	}

	if config.Configuration.PrivilegedMode && config.Configuration.LXCMode {
		return fmt.Errorf("lxcMode cannot be enabled when privilegedMode is true")
	}
	return nil
}

func validatePort(port string) error {
	_, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("port must be a valid integer: %s", port)
	}
	return nil
}

func validateEmail(email string) error {
	_, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("invalid email address: %s", email)
	}
	return nil
}
