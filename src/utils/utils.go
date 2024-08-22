package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"net/mail"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const loggerFlags = log.Ldate | log.Ltime | log.Lshortfile

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
		LogFileDirectory         string `yaml:"logFileDirectory"`
		LogFileName              string `yaml:"logFileName"`
		Stdout                   bool   `yaml:"stdOut"`
		HealthCron               string `yaml:"healthCron"`
		HealthCronDisable        bool   `yaml:"healthCronDisable"`
		HealthCronWebhookDisable bool   `yaml:"healthCronWebhookDisable"`
		HealthCronSmtpDisable    bool   `yaml:"healthCronSmtpDisable"`
		HealthCheckTimeout       int    `yaml:"healthCheckTimeout"`
		DiscordWebHookDisable    bool   `yaml:"discordWebhookDisable"`
		DiscordWebHookURL        string `yaml:"discordWebhookUrl"`
		LogFileSize              string `yaml:"logFileSize"`
		MaxLogFileKeep           int    `yaml:"maxLogFileKeep"`
		SmtpDisable              bool   `yaml:"smtpDisable"`
		SmtpHost                 string `yaml:"smtpHost"`
		SmtpPort                 string `yaml:"smtpPort"`
		SmtpUsername             string `yaml:"smtpUsername"`
		SmtpPassword             string `yaml:"smtpPassword"`
		SmtpFrom                 string `yaml:"smtpFrom"`
		SmtpTo                   string `yaml:"smtpTo"`
	} `yaml:"configuration"`
}

type CronSchedule struct {
	Minute     string
	Hour       string
	DayOfMonth string
	Month      string
	DayOfWeek  string
}

func ParseHealthCron(expression string) (*CronSchedule, error) {
	fields := strings.Fields(expression)
	if len(fields) != 5 {
		return nil, errors.New("invalid cron expression: must have 5 fields")
	}

	return &CronSchedule{
		Minute:     fields[0],
		Hour:       fields[1],
		DayOfMonth: fields[2],
		Month:      fields[3],
		DayOfWeek:  fields[4],
	}, nil
}

func (c *CronSchedule) Match(t time.Time) bool {
	return c.matchField(c.Minute, t.Minute()) &&
		c.matchField(c.Hour, t.Hour()) &&
		c.matchField(c.DayOfMonth, t.Day()) &&
		c.matchField(c.Month, int(t.Month())) &&
		c.matchField(c.DayOfWeek, int(t.Weekday()))
}

func (c *CronSchedule) matchField(field string, value int) bool {
	if field == "*" {
		return true
	}

	for _, part := range strings.Split(field, ",") {
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			start, _ := strconv.Atoi(rangeParts[0])
			end, _ := strconv.Atoi(rangeParts[1])
			if value >= start && value <= end {
				return true
			}
		} else if strings.Contains(part, "/") {
			stepParts := strings.Split(part, "/")
			step, _ := strconv.Atoi(stepParts[1])
			if value%step == 0 {
				return true
			}
		} else {
			if fieldValue, err := strconv.Atoi(part); err == nil && fieldValue == value {
				return true
			}
		}
	}

	return false
}

func InitCronSchedule(config *Config) (*CronSchedule, error) {
	return ParseHealthCron(config.Configuration.HealthCron)
}

func IsScheduledTime(schedule *CronSchedule) bool {
	return schedule.Match(time.Now())
}

type LogEntry struct {
	Type    string `json:"Type"`
	Message string `json:"Message"`
	Event   string `json:"Event"`
}

type SafeLogger struct {
	mu     sync.Mutex
	logger *log.Logger
	file   *os.File
}

func CheckPrivileges(privilegedMode bool) {
	if privilegedMode != isRunningAsRoot() {
		mode := "privileged"
		userStatus := "unprivileged user"
		if !privilegedMode {
			mode = "unprivileged"
			userStatus = "privileged user"
		}
		panic(fmt.Sprintf("Error running %s mode with %s", mode, userStatus))
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

func (sl *SafeLogger) Log(logType, message, event string) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if sl.logger == nil {
		fmt.Printf("Error: Logger is nil. Message: %s\n", message)
		return
	}

	if logType == "" || message == "" || event == "" {
		fmt.Printf("Error creating log entry: logType, message, and event cannot be empty\n")
		return
	}

	logEntry, err := CreateLogEntry(logType, message, event)
	if err != nil {
		fmt.Printf("Error creating log entry: %v\n", err)
		return
	}

	sl.logger.Print(logEntry)
}

func (sl *SafeLogger) Rotate(newFilePath string) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	newFile, err := os.Create(newFilePath)
	if err != nil {
		return fmt.Errorf("failed to create new log file: %v", err)
	}
	sl.logger.SetOutput(newFile)

	if sl.file != nil {
		if err := sl.file.Close(); err != nil {
			return fmt.Errorf("failed to close current log file: %v", err)
		}
	}
	sl.file = newFile
	return nil
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

func ValidateLogDirectory(directoryPath string) error {
	err := os.MkdirAll(directoryPath, 0750)
	if err != nil {
		return fmt.Errorf("unable to create directory: %s", directoryPath)
	}
	return nil
}

func SetupLogger(stdout bool, logPath, logName string) (*SafeLogger, error) {
	safeLogger := &SafeLogger{
		mu: sync.Mutex{},
	}

	if stdout {
		safeLogger.logger = log.New(os.Stdout, "", loggerFlags)
		safeLogger.file = nil
		return safeLogger, nil
	}

	if err := ValidateLogDirectory(logPath); err != nil {
		return nil, fmt.Errorf("invalid log directory: %v", err)
	}

	fullPath := filepath.Join(logPath, logName)
	file, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	safeLogger.logger = log.New(file, "", loggerFlags)
	safeLogger.file = file
	fmt.Printf("Message: Running with output to logfile: %v\n", fullPath)
	return safeLogger, nil
}

func (sl *SafeLogger) Close() error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if sl.logger == nil {
		return nil
	}

	if file, ok := sl.logger.Writer().(*os.File); ok {
		err := file.Close()
		if err != nil {
			return fmt.Errorf("failed to close log file: %v", err)
		}
	}

	return nil
}

func (sl *SafeLogger) RotateLogFile(logPath string, logName string, maxSize int64, maxFiles int) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	fullPath := filepath.Join(logPath, logName)

	if sl.file == nil {
		return nil
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat log file: %v", err)
	}

	if info.Size() < maxSize {
		return nil
	}

	for i := maxFiles - 1; i > 0; i-- {
		oldName := fmt.Sprintf("%s.%d", fullPath, i)
		newName := fmt.Sprintf("%s.%d", fullPath, i+1)
		if _, err := os.Stat(oldName); !os.IsNotExist(err) {
			if err := os.Rename(oldName, newName); err != nil {
				return fmt.Errorf("failed to rename log file: %v", err)
			}
		}
	}

	newPath := fmt.Sprintf("%s.1", fullPath)
	if err := os.Rename(fullPath, newPath); err != nil {
		return fmt.Errorf("failed to rename current log file: %v", err)
	}

	newFile, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create new log file: %v", err)
	}
	defer func() {
		if newFile != nil {
			closeErr := newFile.Close()
			if closeErr != nil {
				if err != nil {
					err = fmt.Errorf("multiple errors: %v; failed to close new log file: %v", err, closeErr)
				} else {
					err = fmt.Errorf("failed to close new log file: %v", closeErr)
				}
			}
		}
	}()

	if err := sl.file.Close(); err != nil {
		return fmt.Errorf("failed to close old log file: %v", err)
	}

	sl.file = newFile
	sl.logger.SetOutput(newFile)

	fmt.Printf("Message: Rotating logfile and moving to: %v . Previous logfiles available by visiting file.log.2, file.log.3, file.log.4, file.log.5 \n", newPath)
	return nil
}

func ConsoleAndLoggerOutput(logger *SafeLogger, logType string, message string, event string) {
	logger.Log(logType, message, event)
	if logType == "ERROR" {
		fmt.Printf("Error: %s\n", message)
	}
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

func validateICMPField(field, value string, index int) error {
	if value == "" {
		return fmt.Errorf("icmp config at index %d has empty %s", index, field)
	}
	return nil
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
		if err := validateICMPField("address", icmp.Address, i); err != nil {
			return err
		}
		if err := validateICMPField("service", icmp.Service, i); err != nil {
			return err
		}
		if err := validateICMPField("networkZone", icmp.NetworkZone, i); err != nil {
			return err
		}
		if err := validateICMPField("instanceType", icmp.InstanceType, i); err != nil {
			return err
		}

		if icmp.Timeout <= 0 {
			return fmt.Errorf("icmp config at index %d has invalid timeout value (should be positive)", i)
		}
		if icmp.FailureTimeout <= 0 {
			return fmt.Errorf("icmp config at index %d has invalid failureTimeout value (should be positive)", i)
		}
		if icmp.RetryBuffer < 0 {
			return fmt.Errorf("icmp config at index %d has invalid retrybuffer value (should be non-negative)", i)
		}

		if _, exists := addresses[icmp.Address]; exists {
			return fmt.Errorf("icmp config at index %d has duplicate address: %s", i, icmp.Address)
		}
		addresses[icmp.Address] = true
	}
	return nil
}

func validateNumericField(field string, value, minValue int, index int) error {
	if value < minValue {
		return fmt.Errorf("http config at index %d has invalid %s value (should be >= %d)", index, field, minValue)
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
		if err := validateICMPField("address", http.Address, i); err != nil {
			return err
		}
		if err := validateICMPField("service", http.Service, i); err != nil {
			return err
		}
		if err := validateICMPField("networkZone", http.NetworkZone, i); err != nil {
			return err
		}
		if err := validateICMPField("instanceType", http.InstanceType, i); err != nil {
			return err
		}

		if err := validateNumericField("timeout", http.Timeout, 1, i); err != nil {
			return err
		}
		if err := validateNumericField("failureTimeout", http.FailureTimeout, 1, i); err != nil {
			return err
		}
		if err := validateNumericField("retryBuffer", http.RetryBuffer, 0, i); err != nil {
			return err
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
		smtpFields := map[string]string{
			"smtpFrom":     config.Configuration.SmtpFrom,
			"smtpTo":       config.Configuration.SmtpTo,
			"smtpHost":     config.Configuration.SmtpHost,
			"smtpPort":     config.Configuration.SmtpPort,
			"smtpUsername": config.Configuration.SmtpUsername,
			"smtpPassword": config.Configuration.SmtpPassword,
		}

		for field, value := range smtpFields {
			if value == "" {
				return fmt.Errorf("%s cannot be empty when smtpDisable is false", field)
			}
		}

		if err := validateEmail(config.Configuration.SmtpFrom); err != nil {
			return fmt.Errorf("smtpFrom is invalid: %v", err)
		}
		if err := validateEmail(config.Configuration.SmtpTo); err != nil {
			return fmt.Errorf("smtpTo is invalid: %v", err)
		}
		if err := validatePort(config.Configuration.SmtpPort); err != nil {
			return fmt.Errorf("smtpPort is invalid: %v", err)
		}
	}

	if !config.Configuration.Stdout {
		if config.Configuration.LogFileSize == "" {
			return fmt.Errorf("logFileSize cannot be empty when stdOut is false")
		}
		if config.Configuration.MaxLogFileKeep <= 0 {
			return fmt.Errorf("maxLogFileKeep must be greater than 0 when stdOut is false")
		}
	}

	if config.Configuration.HealthCron != "" {
		_, err := ParseHealthCron(config.Configuration.HealthCron)
		if err != nil {
			return fmt.Errorf("invalid cron expression: %v", err)
		}
	}

	if config.Configuration.HealthCheckTimeout <= 0 {
		return fmt.Errorf("healthCheckTimeout must be greater than 0")
	}

	if err := ValidateICMPConfig(config.ICMP); err != nil {
		return fmt.Errorf("ICMP config validation failed: %v", err)
	}
	if err := ValidateHTTPConfig(config.HTTP); err != nil {
		return fmt.Errorf("HTTP config validation failed: %v", err)
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

func ConvertToBytes(logFileSize string) (int64, error) {
	logFileSize = strings.TrimSpace(strings.ToUpper(logFileSize))

	multipliers := map[string]int64{
		"KB": 1024,
		"MB": 1024 * 1024,
	}

	var unit string
	for u := range multipliers {
		if strings.HasSuffix(logFileSize, u) {
			unit = u
			break
		}
	}

	if unit == "" {
		return 0, fmt.Errorf("invalid unit: input must end with KB or MB")
	}

	valueStr := strings.TrimSuffix(logFileSize, unit)
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid or negative number: %s", valueStr)
	}

	return int64(value * float64(multipliers[unit])), nil
}

func ConvertStringToBool(str string) (bool, error) {
	normalizedStr := strings.ToLower(strings.TrimSpace(str))
	switch normalizedStr {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, errors.New("invalid input: must be 'true' or 'false'")
	}
}
