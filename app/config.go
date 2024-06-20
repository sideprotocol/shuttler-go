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
	"github.com/cosmos/go-bip39"
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
	Chain string `toml:"chain"                          comment:"Bitcoin chains: mainnet, testnet, regtest, signet" default:"mainnet" validate:"oneof=mainnet testnet regtest signet"`
	// Bitcoin specific configuration
	RPC         string `toml:"rpc"                      comment:"Bitcoin RPC endpoint"`
	RPCUser     string `toml:"rpcuser"                  comment:"Bitcoin RPC user"`
	RPCPassword string `toml:"rpcpassword"              comment:"Bitcoin RPC password"`
	Protocol    string `toml:"protocol"                 comment:"Bitcoin RPC protocol"`

	ZMQHost string `toml:"zmqhost"                      comment:"Bitcoin ZMQ host"`
	ZMQPort int    `toml:"zmqport"                      comment:"Bitcoin ZMQ port"`

	VaultAddress string `toml:"vault-address"          comment:"Vault address for the transaction"`
	VaultSigner  bool   `toml:"vault-signer"           comment:"Enable vault signer to sign the transaction, only used for testing"`
}

type Side struct {
	// Side specific configuration
	GRPC string `toml:"grpc"                          comment:"Side gRPC endpoint"`
	RPC  string `toml:"rpc"                           comment:"Side RPC endpoint"`
	REST string `toml:"rest"                          comment:"Side REST endpoint"`

	Frequency int    `toml:"frequency"                 comment:"frequency of Side block polling in	seconds"`
	Sender    string `toml:"sender"                    comment:"Side sender address"`
	ChainID   string `toml:"chain-id"                  comment:"Side chain ID"`
	Gas       uint64 `toml:"gas"                       comment:"Side chain gas"`
}

func defaultConfig(network string) *Config {
	return &Config{
		Global: Global{
			LogLevel: "info",
		},
		Bitcoin: Bitcoin{
			Chain:        network,
			RPC:          "signet:38332",
			RPCUser:      "side",
			RPCPassword:  "12345678",
			VaultAddress: "",
			Protocol:     "http",
			ZMQHost:      "signet",
			ZMQPort:      38330,
			VaultSigner:  false,
		},
		Side: Side{
			RPC:       "http://localhost:26657",
			REST:      "http://localhost:1317",
			GRPC:      "localhost:9090",
			Frequency: 6,
			Sender:    "",
			ChainID:   "devnet",
			Gas:       2000000,
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
	DefaultConfigFilePath = DefaultHome + "/config.toml"
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
	return c.homePath + "/config.toml"
}

func (c *ConfigBuilder) InitConfig(m, network string) *Config {
	cfg := defaultConfig(network)

	// Set the sender address
	c.setKeyringPrefix(network)
	hdPath, algo := getKeyType("segwit")

	// init keyring
	cdc := getCodec()
	kb, err := keyring.New(AppName, keyring.BackendTest, c.homePath, nil, cdc)
	if err != nil {
		panic(err)
	}
	mnemonic := m
	if mnemonic == "" {
		entropy, _ := bip39.NewEntropy(128)
		mnemonic, _ = bip39.NewMnemonic(entropy)
	}

	record, err := kb.NewAccount(InternalKeyringName, mnemonic, "", hdPath, algo)
	if err != nil {
		panic(err)
	}
	accAddr, err := record.GetAddress()
	if err != nil {
		panic(err)
	}
	cfg.Side.Sender = accAddr.String()

	println("====================================================")
	println("Mnemonic: ", mnemonic)
	println("Address:  ", accAddr.String())
	println("====================================================")

	out, err := toml.Marshal(cfg)
	if err != nil {
		panic(err)
	}

	os.MkdirAll(c.homePath, 0755)

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
		return c.InitConfig("", "mainnet")
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
	// sdk.GetConfig().SetBech32PrefixForAccount(prefix, prefix+sdk.PrefixPublic)
	sdk.GetConfig().SetBtcChainCfg(ChainParams(chain))
	sdk.GetConfig().Seal()
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
