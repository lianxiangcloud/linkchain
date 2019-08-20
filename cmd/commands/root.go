package commands

import (
	"os"
	"path/filepath"
	"sync"

	cfg "github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/cli"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	config  = cfg.DefaultConfig()
	logger  = log.Root()
	logOnce sync.Once
)

func init() {
	registerFlagsRootCmd(RootCmd)
	logger.SetHandler(log.StdoutHandler)
}

func registerFlagsRootCmd(cmd *cobra.Command) {
	cmd.PersistentFlags().String("log_level", config.LogLevel, "Log level")
	cmd.PersistentFlags().String("log.filename", config.Log.Filename, "Log file name")
	cmd.PersistentFlags().String("log.perm", config.Log.Perm, "Log file perm")
	cmd.PersistentFlags().Bool("log.rotate", config.Log.Rotate, "Support log rotate")
	cmd.PersistentFlags().String("log.rotatePerm", config.Log.RotatePerm, "Rotate file perm")
	cmd.PersistentFlags().Int("log.maxDays", config.Log.MaxDays, "How many old logs to retain")
	cmd.PersistentFlags().Int("log.maxLines", config.Log.MaxLines, "Rotate when the lines reach here")
	cmd.PersistentFlags().Int("log.maxSize", config.Log.MaxSize, "Rotate when the size reach here")
	cmd.PersistentFlags().Bool("log.daily", config.Log.Daily, "Rotate daily")
	cmd.PersistentFlags().Bool("log.hourly", config.Log.Hourly, "Rotate hourly")
	cmd.PersistentFlags().Bool("log.minutely", config.Log.Minutely, "Rotate minutely")
	cmd.PersistentFlags().Int("log.minutes", config.Log.Minutes, "Rotate minutes M where 60 % M == 0")
}

// ParseConfig retrieves the default environment configuration,
// sets up the root and ensures that the root exists
func ParseConfig() (*cfg.Config, error) {
	conf := cfg.DefaultConfig()
	err := viper.Unmarshal(conf)
	if err != nil {
		return nil, err
	}
	conf.SetRoot(conf.RootDir)
	cfg.EnsureRoot(conf.RootDir, conf)
	return conf, err
}

// RootCmd is the root command.
var RootCmd = &cobra.Command{
	Use:   "linkchain",
	Short: "linkchain root command",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		if cmd.Name() == VersionCmd.Name() {
			return nil
		}
		if cmd.Name() == NewConsoleCommand().Name() {
			return nil
		}
		config, err = ParseConfig()
		if err != nil {
			return err
		}
		if !filepath.IsAbs(config.Log.Filename) {
			config.Log.Filename = filepath.Join(config.LogDir(), config.Log.Filename)
		}
		logDir := filepath.Dir(config.Log.Filename)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}
		logOnce.Do(func() {
			rotateHandler, err := log.RotateHandler(config.Log, log.TerminalFormat(false))
			if err != nil {
				logger.Error("rotateHandler err", "err", err)
				return
			}
			logger.SetHandler(rotateHandler)
		})
		logger, err = log.ParseLogLevel(config.LogLevel, logger, cfg.DefaultLogLevel())
		if err != nil {
			return err
		}
		if viper.GetBool(cli.TraceFlag) {
			logger = log.NewTracingLogger(logger)
		}
		logger = logger.With("module", "main")
		return nil
	},
}
