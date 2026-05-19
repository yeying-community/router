package common

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/yeying-community/router/common/logger"
)

var (
	Port         = flag.Int("port", 3011, "the listening port")
	PrintVersion = flag.Bool("version", false, "print version and exit")
	PrintHelp    = flag.Bool("help", false, "print help and exit")
	LogDir       = flag.String("log-dir", "./logs", "specify the log directory")
	ConfigPath   = flag.String("config", "./config.yaml", "path to the YAML config file")
)

func printHelp() {
	fmt.Println("Router " + Version + " - All in router service for OpenAI API.")
	fmt.Println("Copyright (C) 2023 JustSong. All rights reserved.")
	fmt.Println("GitHub: https://github.com/yeying-community/router")
	fmt.Println("Usage: router [--config <config file>] [--port <port>] [--log-dir <log directory>] [--version] [--help]")
}

func Init() {
	flag.Parse()

	if *PrintVersion {
		fmt.Println(Version)
		os.Exit(0)
	}

	if *PrintHelp {
		printHelp()
		os.Exit(0)
	}

	appConfig, err := LoadAppConfig(*ConfigPath)
	if err != nil {
		log.Fatal(err)
	}
	if err = ApplyAppConfig(appConfig, isFlagPassed("port"), isFlagPassed("log-dir")); err != nil {
		log.Fatal(err)
	}

	if *LogDir != "" {
		*LogDir, err = filepath.Abs(*LogDir)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := os.Stat(*LogDir); os.IsNotExist(err) {
			err = os.MkdirAll(*LogDir, 0777)
			if err != nil {
				log.Fatal(err)
			}
		}
		logger.LogDir = *LogDir
	}
}

func isFlagPassed(name string) bool {
	visited := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			visited = true
		}
	})
	return visited
}
