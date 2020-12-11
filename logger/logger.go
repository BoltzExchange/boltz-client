package logger

import (
	"fmt"
	"log"
	"os"

	"github.com/google/logger"
)

var prefix string

func InitLogger(logPath string, logPrefix string) {
	prefix = logPrefix

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)

	if err != nil {
		PrintFatal("Could not open log file: %s", err)
	}

	logger.Init("boltz-lnd", true, false, file)
	logger.SetFlags(log.LstdFlags)

	Info("Initialized logger")
}

func Fatal(message string) {
	logger.Fatal(prefix + message)
}

func Error(message string) {
	logger.Error(prefix + message)
}

func Warning(message string) {
	logger.Warning(prefix + message)
}

func Info(message string) {
	logger.Info(prefix + message)
}

func PrintFatal(format string, a ...interface{}) {
	fmt.Printf(format+"\n", a...)
	os.Exit(1)
}
