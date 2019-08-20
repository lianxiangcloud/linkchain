package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	flog "github.com/lianxiangcloud/linkchain/libs/log"
	cfg "github.com/lianxiangcloud/linkchain/config"
)

var (
	log = flog.Root()
)

func logInit() {
	logConfig := cfg.DefaultRotateConfig()
	logConfig.Daily    = true
	logConfig.Hourly   = false
	logConfig.Filename = "logs/lk_blockagent.log"
	if !filepath.IsAbs(logConfig.Filename) {
		dir, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		logConfig.Filename = filepath.Join(dir, logConfig.Filename)
	}
	logDir := filepath.Dir(logConfig.Filename)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		panic(err)
	}
	rotateHandler, err := flog.RotateHandler(logConfig, flog.TerminalFormat(false))
	if err != nil {
		panic(err)
	}
	log.SetHandler(rotateHandler)
	log, err = flog.ParseLogLevel("*:trace", log, "info")
	if err != nil {
		panic(err)
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: ./metrics_collector <config_file_path>")
		os.Exit(1)
	}
	configPath := os.Args[1]

	var configs lkBlockAgentConfigs
	if _, err := toml.DecodeFile(configPath, &configs); err != nil {
		fmt.Println("toml.DecodeFile failed.", "err", err)
		os.Exit(1)
	}
	logInit()

	runLKBlockAgent(&configs)
}
