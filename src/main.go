package main

import (
	"fmt"
	"github.com/go-ping/ping"
	"github.com/somememoryspace/inframon/src/utils"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	MASTER_PRINT_BOOL = true
	LOGGER            = utils.SetupLogger("./logs", "runtime.log")
	MASTER_TIMEOUT    = 60
)

func pingICMP(address string) time.Duration {
	pinger, err := ping.NewPinger(address)
	if err != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("could not connect to address: (%s)", address), "error", MASTER_PRINT_BOOL)
		return 0
	}
	pinger.Count = 1
	pinger.Timeout = 2 * time.Second
	err = pinger.Run()
	if err != nil {
		utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("could not connect to address: (%s)", address), "error", MASTER_PRINT_BOOL)
		return 0
	}
	stats := pinger.Statistics()
	return stats.AvgRtt
}

func pingTask(address string, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		latency := pingICMP(address)
		utils.ConsoleAndLoggerOutput(LOGGER, fmt.Sprintf("pinged address %s with latency %v", address, latency), "info", MASTER_PRINT_BOOL)
		time.Sleep(time.Duration(MASTER_TIMEOUT) * time.Second)
	}
}

func main() {

	// Setup Logger
	utils.ConsoleAndLoggerOutput(LOGGER, "runtime start", "info", MASTER_PRINT_BOOL)

	//Test Ping
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup
	wg.Add(2)
	go pingTask("127.0.0.1", &wg)

	// Load Testing
	go pingTask("10.10.100.20", &wg)
	go pingTask("10.10.100.21", &wg)
	go pingTask("10.10.100.22", &wg)
	go pingTask("10.10.100.100", &wg)
	go pingTask("10.10.100.109", &wg)
	go pingTask("10.10.100.110", &wg)

	go pingTask("10.10.200.20", &wg)
	go pingTask("10.10.200.21", &wg)
	go pingTask("10.10.200.22", &wg)
	go pingTask("10.10.200.30", &wg)

	go pingTask("10.10.150.51", &wg)
	go pingTask("10.10.150.52", &wg)
	go pingTask("10.10.150.53", &wg)
	go pingTask("10.10.150.61", &wg)
	go pingTask("10.10.150.71", &wg)

	<-signalChannel
	utils.ConsoleAndLoggerOutput(LOGGER, "termination signal received. exiting...", "info", MASTER_PRINT_BOOL)
}
