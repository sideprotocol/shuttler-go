package bitcoin

import (
	"testing"

	"github.com/sideprotocol/shuttler/app"
)

func Test_Relayer(t *testing.T) {

	cfg := app.Config{
		Global: app.Global{
			LogLevel: "info",
		},
		Bitcoin: app.Bitcoin{
			RPC:         "149.28.156.79:18332",
			RPCUser:     "side",
			RPCPassword: "12345678",
			Frequency:   10 * 60 * 60,
			Sender:      "",
		},
		Side: app.Side{
			RPC:       "http://localhost:26657",
			REST:      "http://localhost:1317",
			Frequency: 6,
			Sender:    "",
		},
	}

	relayer := NewBTCRelayer(cfg, nil)
	defer relayer.Shutdown()

	err := relayer.SyncHeader()
	if err != nil {
		t.Error("Failed to sync header", err)
	}

}
