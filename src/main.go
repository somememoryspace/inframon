package main

import (
	"github.com/somememoryspace/inframon/src/utils"
)

func main() {
	// Setup Logger
	logger := utils.SetupLogger("./logs", "runtime.log")
	logger.Println(utils.Create_log_entry("runtime start", "info"))
}
