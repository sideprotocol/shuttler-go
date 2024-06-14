package app

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"

	btclightclient "github.com/sideprotocol/side/x/btcbridge/types"
)

type EncodingConfig struct {
	InterfaceRegistry types.InterfaceRegistry
	Codec             codec.Codec
	TxConfig          client.TxConfig
	Amino             *codec.LegacyAmino
}

func MakeEncodingConfig() EncodingConfig {
	aminoCodec := codec.NewLegacyAmino()
	interfaceRegistry := types.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(interfaceRegistry)
	cdc.InterfaceRegistry().RegisterImplementations((*sdk.Msg)(nil), &btclightclient.MsgSubmitBlockHeaderRequest{})
	cdc.InterfaceRegistry().RegisterImplementations((*sdk.Msg)(nil), &btclightclient.MsgUpdateSendersRequest{})

	encCfg := EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Codec:             cdc,
		TxConfig:          tx.NewTxConfig(cdc, tx.DefaultSignModes),
		Amino:             aminoCodec,
	}

	return encCfg
}
