package boltz_lnd

import (
	"fmt"
	"log"
	"os"

	"github.com/google/logger"
)

func InitLogger(logPath string) {
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)

	if err != nil {
		printFatal("Could not open log file: %s", err)
	}

	logger.Init("channel-bot", true, false, file)
	logger.SetFlags(log.LstdFlags)

	logger.Info("Initialized logger")
}

func printFatal(format string, a ...interface{}) {
	fmt.Printf(format+"\n", a...)
	os.Exit(1)
}
