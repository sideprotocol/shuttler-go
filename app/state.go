package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/btcsuite/btcd/wire"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// AppState is the modifiable state of the application.
type State struct {
	// Log is the root logger of the application.
	// Consumers are expected to store and use local copies of the logger
	// after modifying with the .With method.
	Log *zap.Logger

	Viper *viper.Viper

	HomePath    string
	Debug       bool
	Config      *Config
	TrustHeader wire.BlockHeader
}

// NewState creates a new State object.
func NewAppState(home string) *State {
	h := home
	if h == "" {
		h = DefaultHome
	}
	return &State{
		Viper:    viper.New(),
		HomePath: h,
	}
}

func (a *State) InitLogger(configLogLevel string) error {
	// logLevel := a.Viper.GetString("log-level")
	// if a.Viper.GetBool("debug") {
	// 	logLevel = "debug"
	// } else if logLevel == "" {
	// 	logLevel = configLogLevel
	// }
	// log, err := newRootLogger(a.Viper.GetString("log-format"), logLevel)
	// if err != nil {
	// 	return err
	// }

	a.Log = zap.Must(zap.NewDevelopment())
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

	return nil
}

func (a *State) ReadCA() ([]byte, error) {
	return os.ReadFile(filepath.Join(a.HomePath, CA_FILE))
}
