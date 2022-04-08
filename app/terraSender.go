package app

import sdk "github.com/cosmos/cosmos-sdk/types"

func (app *TerraApp) SendTerraAccount(ctx sdk.Context) {
	wallet := app.GetWallets()
	walletAddr, _ := sdk.AccAddressFromBech32(wallet["terraWallet"])
	account := app.AccountKeeper.GetAccount(ctx, walletAddr)
	encodingConfig := MakeEncodingConfig()
	value, _ := encodingConfig.Marshaler.MarshalInterface(account)
	app.ZmqSendMessage("TerraAccount", value)
}
