package app

import (
	"testing"
)

func Test_Config(t *testing.T) {

	cb := NewConfigBuilder(t.TempDir())
	cfg := cb.InitConfig("")
	if cfg.Global.LogLevel != "info" {
		t.Errorf("Expected info, got %s", cfg.Global.LogLevel)
	}

	cfg2 := cb.LoadConfigFile()

	if cfg.Global.LogLevel != cfg2.Global.LogLevel {
		t.Errorf("Expected %s, got %s", cfg.Global.LogLevel, cfg2.Global.LogLevel)
	}

	if cfg.Bitcoin.RPC != cfg2.Bitcoin.RPC {
		t.Errorf("Expected %s, got %s", cfg.Bitcoin.RPC, cfg2.Bitcoin.RPC)
	}

	if cfg.Bitcoin.RPCUser != cfg2.Bitcoin.RPCUser {
		t.Errorf("Expected %s, got %s", cfg.Bitcoin.RPCUser, cfg2.Bitcoin.RPCUser)
	}

}
