package logger

import (
	"fmt"
	"github.com/fatih/color"
	"gopkg.in/natefinch/lumberjack.v2"
	"log"
	"os"
	"strings"
	"time"
)

type LogLevel int

const (
	levelFatal LogLevel = iota
	levelError
	levelWarn
	levelInfo
	levelDebug
	levelSilly

	loggerFlags    = 0
	termTimeFormat = "2006-01-02 15:04:05.000"
)

var (
	isDisabled = true

	logLevel = levelInfo

	fatalPrefix = color.RedString("FATAL")
	errorPrefix = color.RedString("ERROR")
	warnPrefix  = color.YellowString("WARN ")
	infoPrefix  = color.GreenString("INFO ")
	debugPrefix = color.CyanString("DEBUG")
	sillyPrefix = "SILLY"

	consoleLogger = log.New(os.Stdout, "", loggerFlags)
	fileLogger    *log.Logger
)

type Options struct {
	*lumberjack.Logger
	Level string
}

// Init set logfile to "" to disable the file logger
func Init(options Options) {
	isDisabled = false

	if options.Logger != nil && options.Filename != "" {
		fileLogger = log.New(options.Logger, "", loggerFlags)
	}

	parseLogLevel(options.Level)
}

func parseLogLevel(level string) {
	switch strings.ToLower(level) {
	case "fatal":
		logLevel = levelFatal

	case "error":
		logLevel = levelError

	case "info":
		logLevel = levelInfo

	case "debug":
		logLevel = levelDebug

	case "":
		fallthrough
	case "silly":
		logLevel = levelSilly

	default:
		Warnf("Unknown log level \"%s\"; using default", level)
	}

}

func Fatal(message string) {
	write(consoleLogger, levelFatal, fatalPrefix, message)
	os.Exit(1)
}

func Fatalf(message string, args ...any) {
	Fatal(fmt.Sprintf(message, args...))
}

func Error(message string) {
	write(consoleLogger, levelError, errorPrefix, message)
}

func Errorf(message string, args ...any) {
	Error(fmt.Sprintf(message, args...))
}

func Warn(message string) {
	write(consoleLogger, levelWarn, warnPrefix, message)
}

func Warnf(message string, args ...any) {
	Warn(fmt.Sprintf(message, args...))
}

func Info(message string) {
	write(consoleLogger, levelInfo, infoPrefix, message)
}

func Infof(message string, args ...any) {
	Info(fmt.Sprintf(message, args...))
}

func Debug(message string) {
	write(consoleLogger, levelDebug, debugPrefix, message)
}

func Debugf(message string, args ...any) {
	Debug(fmt.Sprintf(message, args...))
}

func Silly(message string) {
	write(consoleLogger, levelSilly, sillyPrefix, message)
}

func Sillyf(message string, args ...any) {
	Silly(fmt.Sprintf(message, args...))
}

func write(l *log.Logger, level LogLevel, prefix string, msg string) {
	if isDisabled {
		return
	}

	if level > logLevel {
		return
	}

	logLine := fmt.Sprintf("%s [%s] %s\n", time.Now().Format(termTimeFormat), prefix, msg)

	l.Print(logLine)

	if fileLogger != nil && prefix != fatalPrefix {
		fileLogger.Print(logLine)
	}
}
