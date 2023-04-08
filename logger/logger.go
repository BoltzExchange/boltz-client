package logger

import (
	"fmt"
	"log"
	"os"

	"github.com/google/logger"
)

var (
	prefix      string
	initialized bool
)

func InitLogger(logPath string, logPrefix string) {
	prefix = logPrefix

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)

	if err != nil {
		PrintFatal("Could not open log file: %s", err)
	}

	logger.Init("boltz-lnd", true, false, file)
	logger.SetFlags(log.LstdFlags)

	initialized = true

	Info("Initialized logger")
}

func Fatal(message string) {
	if !initialized {
		return
	}

	logger.Fatal(prefix + message)
}

func Fatalf(message string, args ...any) {
	if !initialized {
		return
	}

	logger.Fatalf(prefix+message, args...)
}

func Error(message string) {
	if !initialized {
		return
	}

	logger.Error(prefix + message)
}

func Errorf(message string, args ...any) {
	if !initialized {
		return
	}

	logger.Errorf(prefix+message, args...)
}

func Warning(message string) {
	if !initialized {
		return
	}

	logger.Warning(prefix + message)
}

func Warningf(message string, args ...any) {
	if !initialized {
		return
	}

	logger.Warningf(prefix+message, args...)
}

func Info(message string) {
	if !initialized {
		return
	}

	logger.Info(prefix + message)
}

func Infof(message string, args ...any) {
	if !initialized {
		return
	}

	logger.Infof(prefix+message, args...)
}

func PrintFatal(format string, a ...interface{}) {
	fmt.Printf(format+"\n", a...)
	os.Exit(1)
}
