package app

import (
	"fmt"
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

	Vaults       []string `toml:"vault-addresses"          comment:"Vault address list for the transaction"`
	LocalSigning bool     `toml:"local-signing"           comment:"Enable local vault signers to sign the transaction, only used for testing"`
}

type Side struct {
	// Side specific configuration
	GRPC string `toml:"grpc"                          comment:"Side gRPC endpoint"`
	RPC  string `toml:"rpc"                           comment:"Side RPC endpoint"`
	REST string `toml:"rest"                          comment:"Side REST endpoint"`

	Frequency int    `toml:"frequency"                 comment:"frequency of Side block polling in seconds"`
	Sender    string `toml:"sender"                    comment:"Side sender address"`
	ChainID   string `toml:"chain-id"                  comment:"Side chain ID"`
	Gas       uint64 `toml:"gas"                       comment:"Side chain gas"`

	Retries int `toml:"retries"                        comment:"retry count on failed"`
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
			Protocol:     "http",
			ZMQHost:      "signet",
			ZMQPort:      38330,
			Vaults:       []string{},
			LocalSigning: false,
		},
		Side: Side{
			RPC:       "http://localhost:26657",
			REST:      "http://localhost:1317",
			GRPC:      "localhost:9090",
			Frequency: 6,
			Sender:    "",
			ChainID:   "devnet",
			Gas:       2000000,
			Retries:   5,
		},
	}
}

const (
	AppName             = "shuttler"
	InternalKeyringName = "side"
	VaultKeyPrefix      = "vault"
	DefaultKeyType      = "segwit"
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

func (c *ConfigBuilder) InitConfig(m string, mVaults []string, network string, keyType string) *Config {
	cfg := defaultConfig(network)

	// Set the sender address
	c.setKeyringPrefix(network)

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

	if len(keyType) == 0 {
		keyType = DefaultKeyType
	}

	accAddr, err := tryCreateKey(kb, InternalKeyringName, mnemonic, keyType)
	if err != nil {
		panic(err)
	}

	cfg.Side.Sender = accAddr.String()

	println("====================================================")
	println("Mnemonic: ", mnemonic)
	println("Address:  ", accAddr.String())
	println("====================================================")

	if err := c.addVaults(cfg, kb, mVaults, keyType); err != nil {
		panic(err)
	}

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

func (c *ConfigBuilder) addVaults(cfg *Config, kb keyring.Keyring, mVaults []string, keyType string) error {
	for i, m := range mVaults {
		keyName := fmt.Sprintf("%s%d", VaultKeyPrefix, i+1)

		accAddr, err := tryCreateKey(kb, keyName, m, keyType)
		if err != nil {
			return err
		}

		cfg.Bitcoin.Vaults = append(cfg.Bitcoin.Vaults, accAddr.String())

		println("====================================================")
		println("Vault ", i+1, "Mnemonic: ", m)
		println("Address:  ", accAddr.String())
		println("====================================================")
	}

	if len(mVaults) > 0 {
		cfg.Bitcoin.LocalSigning = true
	}

	return nil
}

func (c *ConfigBuilder) LoadConfigFile() *Config {

	// check if config file exists
	_, err := os.Stat(c.ConfigFilePath())
	if os.IsNotExist(err) {
		return c.InitConfig("", nil, "mainnet", "")
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
	case "taproot":
		return "m/86'/0'/0'/0/0", hd.Taproot
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

// tryCreateKey creates the key if the derived address does not exist yet
func tryCreateKey(kb keyring.Keyring, keyName string, mnemonic string, keyType string) (sdk.AccAddress, error) {
	// check if the address already exists

	hdPath, algo := getKeyType(keyType)

	derivedPriv, err := algo.Derive()(mnemonic, "", hdPath)
	if err != nil {
		return nil, err
	}

	privKey := algo.Generate()(derivedPriv)
	address := sdk.AccAddress(privKey.PubKey().Address())

	if _, err := kb.KeyByAddress(address); err == nil {
		return address, nil
	}

	// delete the key if the key name already exists
	kb.Delete(keyName)

	// create new account
	record, err := kb.NewAccount(keyName, mnemonic, "", hdPath, algo)
	if err != nil {
		return nil, err
	}

	return record.GetAddress()
}
