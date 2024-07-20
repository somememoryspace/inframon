package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type LogEntry struct {
	Message string `json:"message"`
	Event   string `json:"event"`
}

func CreateLogEntry(Message string, Event string) string {
	logEntry := LogEntry{
		Message: Message,
		Event:   Event,
	}
	jsonData, err := json.Marshal(logEntry)
	if err != nil {
		log.Fatalf("Error marshalling to JSON: %v", err)
	}
	//Return
	return string(jsonData)
}

func ValidateLogDirectory(directoryPath string) {
	fmt.Println(CreateLogEntry(fmt.Sprintf("validating %s directory available... skipping if available", directoryPath), "info"))
	err := os.MkdirAll(directoryPath, os.ModePerm)
	if err != nil {
		panic(CreateLogEntry(fmt.Sprintf("unable to create directory: %s", directoryPath), "panic"))
	}
	fmt.Println(CreateLogEntry(fmt.Sprintf("validated %s directory is available", directoryPath), "info"))
}

func SetupLogger(directoryPath string, logName string) *log.Logger {
	ValidateLogDirectory(directoryPath)
	logFilePath := fmt.Sprintf("%s/runtime.log", directoryPath)
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(CreateLogEntry(fmt.Sprintf("unable to open log file: %s", logFilePath), "panic"))
	}
	logger := log.New(file, "event: ", log.Ldate|log.Ltime|log.Lshortfile)
	return logger
}

func ConsoleAndLoggerOutput(logger *log.Logger, Message string, Event string, ConsoleOut bool) {
	logger.Println(CreateLogEntry(Message, Event))
	if ConsoleOut {
		fmt.Println(CreateLogEntry(Message, Event))
	}
}
