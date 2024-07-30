package utils

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"net/mail"
	"os"
)

type Config struct {
	ICMP []struct {
		Address      string `yaml:"address"`
		Service      string `yaml:"service"`
		Timeout      int    `yaml:"timeout"`
		RetryBuffer  int    `yaml:"retrybuffer"`
		NetworkZone  string `yaml:"networkZone"`
		InstanceType string `yaml:"instanceType"`
	} `yaml:"icmp"`

	HTTP []struct {
		Address      string `yaml:"address"`
		Service      string `yaml:"service"`
		Timeout      int    `yaml:"timeout"`
		SkipVerify   bool   `yaml:"skipVerify"`
		RetryBuffer  int    `yaml:"retrybuffer"`
		NetworkZone  string `yaml:"networkZone"`
		InstanceType string `yaml:"instanceType"`
	} `yaml:"http"`

	Configuration struct {
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
	} `yaml:"configuration"`
}

type Message struct {
	Content string         `json:"content,omitempty"`
	Embeds  []DiscordEmbed `json:"embeds,omitempty"`
}

type DiscordEmbed struct {
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Color       int            `json:"color,omitempty"`
	Fields      []DiscordField `json:"fields,omitempty"`
}

type DiscordField struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
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
	fmt.Println(fmt.Sprintf("message: system ::::: validating %s directory available... skipping if available", directoryPath), "info")
	err := os.MkdirAll(directoryPath, os.ModePerm)
	if err != nil {
		panic(fmt.Sprintf("message: system ::::: unable to create directory: %s", directoryPath))
	}
	fmt.Println(fmt.Sprintf("message: system ::::: validated %s directory available", directoryPath), "info")
}

func SetupLogger(directoryPath string, logName string) *log.Logger {
	ValidateLogDirectory(directoryPath)
	logFilePath := fmt.Sprintf("%s/runtime.log", directoryPath)
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(fmt.Sprintf("message: system ::::: unable to open log file: %s", logFilePath))
	}
	logger := log.New(file, "event: ", log.Ldate|log.Ltime|log.Lshortfile)
	return logger
}

func ConsoleAndLoggerOutput(logger *log.Logger, Type string, Message string, Event string, ConsoleOut bool) {
	logEntry, err := CreateLogEntry(Type, Message, Event)
	if err != nil {
		fmt.Printf("system ::::: error creating log entry: %v\n", err)
		return
	}
	logger.Println(logEntry)
	if ConsoleOut {
		fmt.Printf("message: %s ::::: %s ::::: event: %s \n", Type, Message, Event)
	}
}

func ValidateICMPConfig(icmpConfig []struct {
	Address      string `yaml:"address"`
	Service      string `yaml:"service"`
	Timeout      int    `yaml:"timeout"`
	RetryBuffer  int    `yaml:"retrybuffer"`
	NetworkZone  string `yaml:"networkZone"`
	InstanceType string `yaml:"instanceType"`
}) error {
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
		if icmp.RetryBuffer < 0 {
			return fmt.Errorf("icmp config at index %d has invalid retrybuffer value", i)
		}
		if icmp.NetworkZone == "" {
			return fmt.Errorf("icmp config at index %d has empty networkZone", i)
		}
		if icmp.InstanceType == "" {
			return fmt.Errorf("icmp config at index %d has empty instanceType", i)
		}
	}
	return nil
}

func ValidateHTTPConfig(httpConfig []struct {
	Address      string `yaml:"address"`
	Service      string `yaml:"service"`
	Timeout      int    `yaml:"timeout"`
	SkipVerify   bool   `yaml:"skipVerify"`
	RetryBuffer  int    `yaml:"retrybuffer"`
	NetworkZone  string `yaml:"networkZone"`
	InstanceType string `yaml:"instanceType"`
}) error {
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
		if http.RetryBuffer < 0 {
			return fmt.Errorf("http config at index %d has invalid retrybuffer value", i)
		}
		if http.NetworkZone == "" {
			return fmt.Errorf("http config at index %d has empty networkZone", i)
		}
		if http.InstanceType == "" {
			return fmt.Errorf("http config at index %d has empty instanceType", i)
		}
	}
	return nil
}

func ValidateConfiguration(config *Config) error {
	if !config.Configuration.DiscordWebHookDisable {
		if config.Configuration.DiscordWebHookURL == "" {
			return fmt.Errorf("discordWebhookUrl cannot be empty when discordWebhookDisable is false")
		}
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
	if config.Configuration.HealthCheckTimeout <= 0 {
		return fmt.Errorf("healthCheckTimeout must be greater than 0")
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
