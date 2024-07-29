package utils

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"strings"
)

type Config struct {
	ICMP []struct {
		IP          string   `yaml:"ip"`
		Hostname    string   `yaml:"hostname"`
		Timeout     int      `yaml:"timeout"`
		RetryBuffer int      `yaml:"retrybuffer"`
		Tags        []string `yaml:"tags"`
	} `yaml:"icmp"`

	HTTP []struct {
		FQDN        string   `yaml:"fqdn"`
		Hostname    string   `yaml:"hostname"`
		Timeout     int      `yaml:"timeout"`
		RetryBuffer int      `yaml:"retrybuffer"`
		SkipVerify  bool     `yaml:"skipverify"`
		Tags        []string `yaml:"tags"`
	} `yaml:"http"`

	Configuration struct {
		LogFileDirectory      string `yaml:"logFileDirectory"`
		LogFileName           string `yaml:"logFileName"`
		Stdout                bool   `yaml:"stdOut"`
		HealthCheckTimeout    int    `yaml:"healthCheckTimeout"`
		DiscordWebHookDisable bool   `yaml:"discordWebhookDisable"`
		DiscordWebHookURL     string `yaml:"discordWebhookUrl"`
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

func CreateLogEntry(Message string, Event string) string {
	logEntry := LogEntry{
		Message: Message,
		Event:   Event,
	}
	jsonData, err := json.Marshal(logEntry)
	if err != nil {
		panic(fmt.Sprintf("unable to serialize to json: %v", err))
	}
	return string(jsonData)
}

func FormatTags(tags []string) string {
	return strings.Join(tags, ", ")
}

func ValidateLogDirectory(directoryPath string) {
	fmt.Println(CreateLogEntry(fmt.Sprintf("syst ::::: validating %s directory available... skipping if available", directoryPath), "info"))
	err := os.MkdirAll(directoryPath, os.ModePerm)
	if err != nil {
		panic(CreateLogEntry(fmt.Sprintf("syst ::::: unable to create directory: %s", directoryPath), "panic"))
	}
	fmt.Println(CreateLogEntry(fmt.Sprintf("syst ::::: validated %s directory is available", directoryPath), "info"))
}

func SetupLogger(directoryPath string, logName string) *log.Logger {
	ValidateLogDirectory(directoryPath)
	logFilePath := fmt.Sprintf("%s/runtime.log", directoryPath)
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(CreateLogEntry(fmt.Sprintf("syst ::::: unable to open log file: %s", logFilePath), "panic"))
	}
	logger := log.New(file, "event: ", log.Ldate|log.Ltime|log.Lshortfile)
	return logger
}

func ConsoleAndLoggerOutput(logger *log.Logger, Message string, Event string, ConsoleOut bool) {
	logger.Println(CreateLogEntry(Message, Event))
	if ConsoleOut {
		CreateLogEntry(Message, Event)
		fmt.Printf("message: %s ::::: event: %s \n", Message, Event)
	}
}
