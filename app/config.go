package app

import (
	"os"
	"path/filepath"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Global      Global  `toml:"global"`
	Bitcoin     Bitcoin `toml:"bitcoin"`
	Side        Side    `toml:"side"`
	FromAddress string  `toml:"from-address" comment:"from address for the transaction"`
}

type Global struct {
	LogLevel string `toml:"log-level"                   comment:"log level of the daemon"`
}

type Bitcoin struct {
	Chain string `toml:"chain"                         comment:"Bitcoin chains: mainnet, testnet, regtest, signet" default:"mainnet" validate:"oneof=mainnet testnet regtest signet"`
	// Bitcoin specific configuration
	RPC         string `toml:"rpc"                           comment:"Bitcoin RPC endpoint"`
	RPCUser     string `toml:"rpcuser"                   comment:"Bitcoin RPC user"`
	RPCPassword string `toml:"rpcpassword"           comment:"Bitcoin RPC password"`
	Protocol    string `toml:"protocol"                  comment:"Bitcoin RPC protocol"`

	ZMQHost string `toml:"zmqhost"                           comment:"Bitcoin ZMQ endpoint"`
	ZMQPort int    `toml:"zmqport"                      comment:"Bitcoin ZMQ port"`

	Frequency int    `toml:"frequency"                  comment:"frequency of Bitcoin block polling in seconds"`
	Sender    string `toml:"sender"                     comment:"Bitcoin sender address"`
}

type Side struct {
	// Side specific configuration
	GRPC string `toml:"grpc"                          comment:"Side gRPC endpoint"`
	RPC  string `toml:"rpc"                           comment:"Side RPC endpoint"`
	REST string `toml:"rest"                          comment:"Side REST endpoint"`

	Frequency int    `toml:"frequency"                  comment:"frequency of Side block polling in	seconds"`
	Sender    string `toml:"sender"                     comment:"Side sender address"`
	ChainID   string `toml:"chain-id"                  comment:"Side chain ID"`
	Gas       uint64 `toml:"gas"                       comment:"Side chain gas"`
}

func defaultConfig() *Config {
	return &Config{
		Global: Global{
			LogLevel: "info",
		},
		Bitcoin: Bitcoin{
			Chain:       "mainnet",
			RPC:         "signet:18332",
			RPCUser:     "side",
			RPCPassword: "12345678",
			Frequency:   10 * 60 * 60,
			Sender:      "",
			Protocol:    "http",
			ZMQHost:     "signet",
			ZMQPort:     18330,
		},
		Side: Side{
			RPC:       "http://localhost:26657",
			REST:      "http://localhost:1317",
			GRPC:      "localhost:9090",
			Frequency: 6,
			Sender:    "",
			ChainID:   "S2-testnet-1",
		},
	}
}

const (
	AppName             = "shuttler"
	InternalKeyringName = "side"
)

var (
	DefaultHome           = filepath.Join(os.Getenv("HOME"), ".shuttler")
	CA_FILE               = "rpc.cert"
	DefaultConfigFilePath = DefaultHome + "/config/config.toml"
)

type ConfigBuilder struct {
	homePath string
}

func NewConfigBuilder(homePath string) *ConfigBuilder {
	realpath := homePath
	if realpath == "" {
		realpath = DefaultHome
	}

	return &ConfigBuilder{
		homePath: realpath,
	}
}

func (c *ConfigBuilder) ConfigFilePath() string {
	return c.homePath + "/config/config.toml"
}

func (c *ConfigBuilder) InitConfig() *Config {
	cfg := defaultConfig()

	// Set the sender address
	c.setKeyringPrefix(cfg.Bitcoin.Chain)
	hdPath, algo := getKeyType("segwit")

	// init keyring
	cdc := getCodec()
	kb, err := keyring.New(AppName, keyring.BackendTest, c.homePath, nil, cdc)
	if err != nil {
		panic(err)
	}
	record, mnemonic, err := kb.NewMnemonic(InternalKeyringName, keyring.English, hdPath, "", algo)
	if err != nil {
		panic(err)
	}
	accAddr, err := record.GetAddress()
	if err != nil {
		panic(err)
	}
	cfg.Side.Sender = accAddr.String()

	println("mnemonic: ", mnemonic)

	out, err := toml.Marshal(cfg)
	if err != nil {
		panic(err)
	}

	os.MkdirAll(c.homePath+"/config", 0755)

	err = os.WriteFile(c.ConfigFilePath(), out, 0644)
	if err != nil {
		panic(err)
	}

	return cfg
}

func (c *ConfigBuilder) LoadConfigFile() *Config {

	// check if config file exists
	_, err := os.Stat(c.ConfigFilePath())
	if os.IsNotExist(err) {
		return c.InitConfig()
	}

	in, err := os.ReadFile(c.ConfigFilePath())
	if err != nil {
		panic(err)
	}
	cfg := &Config{}
	err = toml.Unmarshal(in, cfg)
	if err != nil {
		panic(err)
	}
	c.setKeyringPrefix(cfg.Bitcoin.Chain)
	return cfg
}

// Set Prefix for the keyring according to the bitcoin chain
func (c *ConfigBuilder) setKeyringPrefix(chain string) {
	// set keyring prefix
	// Set prefix for sender address to bech32
	switch chain {
	case "mainnet":
		sdk.GetConfig().SetBech32PrefixForAccount("bc", "bcpub")
	case "testnet":
		sdk.GetConfig().SetBech32PrefixForAccount("tb", "tbpub")
	}
	sdk.GetConfig().Seal()
}

func (c *ConfigBuilder) validateConfig() error {
	// validate config
	return nil
}

func getCodec() codec.Codec {
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	return codec.NewProtoCodec(registry)
}

func getKeyType(algo string) (string, keyring.SignatureAlgo) {
	switch algo {
	case "segwit":
		return "m/84'/0'/0'/0/0", hd.SegWit
	default:
		return sdk.FullFundraiserPath, hd.Secp256k1
	}
}

func ChainParams(chain string) *chaincfg.Params {
	switch chain {
	case "mainnet":
		return &chaincfg.MainNetParams
	case "testnet":
		return &chaincfg.TestNet3Params
	case "regtest":
		return &chaincfg.RegressionNetParams
	case "signet":
		return &chaincfg.SigNetParams
	default:
		return &chaincfg.MainNetParams
	}
}
