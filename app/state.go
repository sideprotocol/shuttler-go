package app

import (
	"context"
	"fmt"
	"os"
	"time"

	zaplogfmt "github.com/jsternberg/zap-logfmt"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// AppState is the modifiable state of the application.
type State struct {
	// Log is the root logger of the application.
	// Consumers are expected to store and use local copies of the logger
	// after modifying with the .With method.
	Log *zap.Logger

	Viper *viper.Viper

	HomePath string
	Debug    bool
	Config   *Config
}

func (a *State) InitLogger(configLogLevel string) error {
	logLevel := a.Viper.GetString("log-level")
	if a.Viper.GetBool("debug") {
		logLevel = "debug"
	} else if logLevel == "" {
		logLevel = configLogLevel
	}
	log, err := newRootLogger(a.Viper.GetString("log-format"), logLevel)
	if err != nil {
		return err
	}

	a.Log = log
	return nil
}

// loadConfigFile reads config file into a.Config if file is present.
func (a *State) LoadConfigFile(ctx context.Context) error {

	cb := NewConfigBuilder(a.HomePath)
	// unmarshall them into the wrapper struct
	cfg := cb.LoadConfigFile()

	if a.Log == nil {
		a.InitLogger(cfg.Global.LogLevel)
	}

	a.Config = cfg

	// ctx.(ctx, "config", a.config)
	// // retrieve the runtime configuration from the disk configuration.
	// newCfg, err := cfgWrapper.RuntimeConfig(ctx, a)
	// if err != nil {
	// 	return err
	// }

	// // validate runtime configuration
	// if err := newCfg.validateConfig(); err != nil {
	// 	return fmt.Errorf("error parsing chain config: %w", err)
	// }

	// // save runtime configuration in app state
	// a.config = newCfg

	return nil
}

func newRootLogger(format string, logLevel string) (*zap.Logger, error) {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format("2006-01-02T15:04:05.000000Z07:00"))
	}
	config.LevelKey = "lvl"

	var enc zapcore.Encoder
	switch format {
	case "json":
		enc = zapcore.NewJSONEncoder(config)
	case "auto", "console":
		enc = zapcore.NewConsoleEncoder(config)
	case "logfmt":
		enc = zaplogfmt.NewEncoder(config)
	default:
		return nil, fmt.Errorf("unrecognized log format %q", format)
	}

	level := zap.InfoLevel
	switch logLevel {
	case "debug":
		level = zap.DebugLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	case "panic":
		level = zapcore.PanicLevel
	case "fatal":
		level = zapcore.FatalLevel
	}
	return zap.New(zapcore.NewCore(
		enc,
		os.Stderr,
		level,
	)), nil
}
