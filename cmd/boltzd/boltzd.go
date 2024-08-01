package main

import (
	"fmt"
	"github.com/BoltzExchange/boltz-client/config"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/rpcserver"
	"github.com/BoltzExchange/boltz-client/utils"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// TODO: close dangling channels

func main() {
	defaultDataDir, err := utils.GetDefaultDataDir()

	if err != nil {
		fmt.Println("Could not get home directory: " + err.Error())
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(defaultDataDir)
	if err != nil {
		fmt.Println("Could not load config: " + err.Error())
		os.Exit(1)
	}

	logger.Init(cfg.LogFile, cfg.LogLevel)

	formattedCfg, err := utils.FormatJson(cfg)

	if err != nil {
		logger.Fatal("Could not format config: " + err.Error())
	}

	logger.Info("Parsed config and CLI arguments: " + formattedCfg)

	if strings.HasSuffix(defaultDataDir, "boltz-lnd") {
		logger.Warn("You still have data in the .boltz-lnd folder - please rename to .boltz")
	}

	rpc := rpcserver.NewRpcServer(cfg)
	if err := rpc.Init(); err != nil {
		logger.Fatalf("Could not initialize Server: %v", err)
	}
	errChannel := rpc.Start()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)
	go func() {
		<-sigc
		logger.Info("Received shutdown signal, stopping server...")
		if err := rpc.Stop(); err != nil {
			logger.Fatal("Could not stop server: " + err.Error())
		}
	}()

	err = <-errChannel

	if err != nil {
		logger.Fatal("Could not start gRPC server: " + err.Error())
	}
}
