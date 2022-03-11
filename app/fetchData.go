package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/tendermint/tendermint/crypto/tmhash"
	wasmexported "github.com/terra-money/core/x/wasm/exported"
	wasmkeeper "github.com/terra-money/core/x/wasm/keeper"
	"github.com/terra-money/core/x/wasm/types"
	"github.com/vmihailenco/msgpack"
)

func changeDenomName(denom string) (assetIn string) {
	if denom == "uusd" {
		assetIn = "UST"
	} else if denom == "uluna" {
		assetIn = "LUNA"
	} else {
		assetIn = ""
	}

	return assetIn
}

func (app *TerraApp) HandleCheckTx(ctx sdk.Context, txBytes []byte) {
	encodingConfig := MakeEncodingConfig()
	decoder := encodingConfig.TxConfig.TxDecoder()
	tx, err := decoder(txBytes)
	if err != nil {
		return
	}

	for _, msg := range tx.GetMsgs() {

		switch msg := msg.(type) {
		case *banktypes.MsgSend:
			app.HandleSendTx(msg)
		case *wasmexported.MsgExecuteContract:
			if msg.Sender == app.GetWallets()["mirrorWallet"] || msg.Sender == app.GetWallets()["terraWallet"] {
				break
			}

			if msg.Contract == app.GetWallets()["terraFactory"] {
				app.HandleFactorySwapTx(msg, txBytes, "terra")
			} else if msg.Contract == app.GetWallets()["astroFactory"] {
				app.HandleFactorySwapTx(msg, txBytes, "astro")
			} else if msg.Contract == app.GetWallets()["mintContract"] {
				data, _ := msg.ExecuteMsg.MarshalJSON()
				app.HandleMintTx(ctx, data, txBytes)
			} else {

				if app.terraToken["reverse"][msg.Contract] != "" || app.terraPair["reverse"][msg.Contract] != "" {
					app.HandleTerraTx(ctx, msg, txBytes)
				}

				if app.mirrorToken["reverse"][msg.Contract] != "" || app.mirrorPair["reverse"][msg.Contract] != "" {
					app.HandleMirrorTx(ctx, msg, txBytes)
				}

				if msg.Contract == app.terraToken["normal"]["AUST"] {
					data, _ := msg.ExecuteMsg.MarshalJSON()

					msgExecute := make(map[string]interface{})
					json.Unmarshal(data, &msgExecute)
					if msgExecute["send"] == nil {
						return
					}

					msg := msgExecute["send"].(map[string]interface{})["msg"].(string)
					decodedData, _ := base64.StdEncoding.DecodeString(msg)
					app.HandleMintTx(ctx, decodedData, txBytes)

				}
			}

			if msg.Contract == app.GetWallets()["terraEnemy"] {
				zmqMessage := make(map[string]interface{})
				zmqMessage["hash"] = fmt.Sprintf("%X", tmhash.Sum(txBytes))
				b, _ := msgpack.Marshal(zmqMessage)
				app.ZmqSendMessage("terraEnemy", b)
			}
		}
	}
}

func (app *TerraApp) HandleSendTx(msg *banktypes.MsgSend) {
	zmqMessage := make(map[string]interface{})
	topic := ""
	if msg.FromAddress == app.GetWallets()["shuttle"] && msg.ToAddress == app.GetWallets()["mirrorWallet"] {
		for _, coin := range msg.Amount {
			zmqMessage["assetName"] = changeDenomName(coin.Denom)
			zmqMessage["amount"] = coin.Amount.Sign()
		}
		topic = "mirrorReceiveShuttle"
	}

	if topic == "" {
		return
	}
	b, _ := msgpack.Marshal(zmqMessage)
	app.ZmqSendMessage(topic, b)
}

func (app *TerraApp) HandleMirrorTx(ctx sdk.Context, msg *types.MsgExecuteContract, txBytes []byte) {
	amount := 0

	topic := ""
	zmqMessage := make(map[string]interface{})
	data, _ := msg.ExecuteMsg.MarshalJSON()
	msgExecute := make(map[string]interface{})
	json.Unmarshal(data, &msgExecute)

	if msgExecute["swap"] != nil {
		assetIn := ""
		for _, coin := range msg.Coins {
			assetIn = changeDenomName(coin.Denom)
			amount = int(coin.Amount.Int64())
		}
		pairName := app.mirrorPair["reverse"][msg.Contract]
		if pairName == "" {
			return
		}

		if msg.Sender == app.GetWallets()["mirrorEnemy"] {
			zmqMessage["assetName"] = pairName
			topic = "mirrorEnemy"
		} else {
			if !app.checkBalance(ctx, "UST", msg.Sender, amount) {
				return
			}
			zmqMessage["data"] = make(map[string]interface{})
			zmqMessage["data"].(map[string]interface{})["pairName"] = pairName
			zmqMessage["data"].(map[string]interface{})["assetIn"] = assetIn
			zmqMessage["data"].(map[string]interface{})["amount"] = amount
			zmqMessage["data"].(map[string]interface{})["maxSpread"] = msgExecute["swap"].(map[string]interface{})["max_spread"]
			zmqMessage["data"].(map[string]interface{})["price"] = msgExecute["swap"].(map[string]interface{})["belief_price"]
			topic = "mirrorSwapStart"
		}
	} else if msgExecute["send"] != nil {
		obj := msgExecute["send"].(map[string]interface{})
		contract := obj["contract"].(string)
		assetName := app.mirrorToken["reverse"][msg.Contract]
		pairName := app.mirrorPair["reverse"][contract]
		if pairName != "" && strings.Contains(pairName, assetName) {
			amount, _ = strconv.Atoi(obj["amount"].(string))

			if msg.Sender == app.GetWallets()["mirrorEnemy"] {
				zmqMessage["assetName"] = pairName
				topic = "mirrorEnemy"
			} else {
				if !app.checkBalance(ctx, assetName, msg.Sender, amount) {
					return
				}
				price, spread := extractPriceAndSpread(obj["msg"].(string))
				zmqMessage["data"] = make(map[string]interface{})
				zmqMessage["data"].(map[string]interface{})["pairName"] = pairName
				zmqMessage["data"].(map[string]interface{})["assetIn"] = pairName
				zmqMessage["data"].(map[string]interface{})["amount"] = amount
				zmqMessage["data"].(map[string]interface{})["maxSpread"] = spread
				zmqMessage["data"].(map[string]interface{})["price"] = price
				topic = "mirrorSwapStart"
			}
		} else {
			return
		}
	} else if msgExecute["transfer"] != nil && msg.Sender == app.GetWallets()["shuttle"] {
		obj := msgExecute["transfer"].(map[string]interface{})
		amountStr := obj["amount"].(string)
		amount, _ := strconv.Atoi(amountStr)
		recipient := obj["recipient"].(string)
		if recipient != app.GetWallets()["mirrorWallet"] {
			return
		}
		assetName := app.mirrorToken["reverse"][msg.Contract]
		zmqMessage["assetName"] = assetName
		zmqMessage["amount"] = amount

		topic = "mirrorReceiveShuttle"
	}

	if topic == "" {
		return
	}
	zmqMessage["type"] = "normal"
	zmqMessage["hash"] = fmt.Sprintf("%X", tmhash.Sum(txBytes))
	b, _ := msgpack.Marshal(zmqMessage)
	app.ZmqSendMessage(topic, b)
}

func extractPriceAndSpread(msg string) (string, string) {
	sDec, _ := base64.StdEncoding.DecodeString(msg)
	temp := make(map[string]map[string]string)
	json.Unmarshal(sDec, &temp)
	swapMsg := temp["swap"]
	return swapMsg["belief_price"], swapMsg["max_spread"]
}

func (app *TerraApp) checkBalance(ctx sdk.Context, assetIn, sender string, amount int) bool {
	walletAddr, _ := sdk.AccAddressFromBech32(sender)
	senderBalance := 0
	if assetIn == "UST" {
		senderBalance = int(app.BankKeeper.GetBalance(ctx, walletAddr, "uusd").Amount.Int64())
	} else if assetIn == "LUNA" {
		senderBalance = int(app.BankKeeper.GetBalance(ctx, walletAddr, "uluna").Amount.Int64())
	} else {
		q := wasmkeeper.NewWasmQuerier(app.WasmKeeper)
		query := make(map[string]interface{})
		query["balance"] = make(map[string]interface{})
		query["balance"].(map[string]interface{})["address"] = sender
		queryJson, _ := json.Marshal(query)
		contractAddress := app.mirrorToken["normal"][assetIn]
		result, err := q.CustomQuery(ctx, contractAddress, queryJson)
		if err != nil {
			fmt.Println(err)
			return false
		}

		jsonData := make(map[string]string)
		json.Unmarshal(result, &jsonData)
		senderBalance, _ = strconv.Atoi(jsonData["balance"])
	}

	if senderBalance < amount {
		return false
	}

	return true
}

func (app *TerraApp) HandleTerraTx(ctx sdk.Context, msg *types.MsgExecuteContract, txBytes []byte) {
	topic := ""
	zmqMessage := make(map[string]interface{})
	data, _ := msg.ExecuteMsg.MarshalJSON()

	assetIn := ""
	amount := 0
	assetName := app.terraToken["reverse"][msg.Contract]

	pairName := ""
	if assetName == "" {
		pairName = app.terraPair["reverse"][msg.Contract]
		pieces := strings.Split(pairName, "-")
		if pieces[0] == "UST" || pieces[0] == "LUNA" {
			assetName = pieces[0]
		} else {
			return
		}
	}

	msgExecute := make(map[string]interface{})
	json.Unmarshal(data, &msgExecute)

	if msgExecute["swap"] != nil {
		for _, coin := range msg.Coins {
			assetIn = changeDenomName(coin.Denom)
			amount = int(coin.Amount.Int64())
		}
		zmqMessage["data"] = make(map[string]interface{})
		zmqMessage["data"].(map[string]interface{})["pairName"] = pairName
		zmqMessage["data"].(map[string]interface{})["assetIn"] = assetIn
		zmqMessage["data"].(map[string]interface{})["amount"] = amount

		topic = "terraSwapStart"
	} else if msgExecute["send"] != nil {
		obj := msgExecute["send"].(map[string]interface{})
		contract := obj["contract"].(string)
		pairName = app.terraPair["reverse"][contract]
		amount, _ = strconv.Atoi(obj["amount"].(string))
		if pairName != "" && strings.Contains(pairName, assetName) {
			assetIn = assetName
			zmqMessage["data"] = make(map[string]interface{})
			zmqMessage["data"].(map[string]interface{})["pairName"] = pairName
			zmqMessage["data"].(map[string]interface{})["assetIn"] = assetIn
			zmqMessage["data"].(map[string]interface{})["amount"] = amount
			topic = "terraSwapStart"
		} else if contract == app.GetWallets()["terraFactory"] {
			zmqMessage["type"] = "terra"
			zmqMessage["msg"] = obj["msg"].(string)
			zmqMessage["amount"] = amount
			topic = "factorySwap"
		} else if contract == app.GetWallets()["astroFactory"] {
			zmqMessage["type"] = "astro"
			zmqMessage["msg"] = obj["msg"].(string)
			zmqMessage["amount"] = amount
			topic = "factorySwap"
		} else {
			return
		}
	} else {
		return
	}
	zmqMessage["hash"] = fmt.Sprintf("%X", tmhash.Sum(txBytes))
	b, _ := msgpack.Marshal(zmqMessage)
	app.ZmqSendMessage(topic, b)
}

func (app *TerraApp) HandleFactorySwapTx(msg *types.MsgExecuteContract, txBytes []byte, types string) {

	zmqMessage := make(map[string]interface{})
	data, _ := msg.ExecuteMsg.MarshalJSON()
	msgExecute := make(map[string]interface{})
	json.Unmarshal(data, &msgExecute)

	amount := 0
	for _, coin := range msg.Coins {
		amount = int(coin.Amount.Int64())
	}
	message := base64.StdEncoding.EncodeToString(data)
	zmqMessage["type"] = types
	zmqMessage["msg"] = message
	zmqMessage["amount"] = amount
	zmqMessage["hash"] = fmt.Sprintf("%X", tmhash.Sum(txBytes))
	topic := "factorySwap"
	b, _ := msgpack.Marshal(zmqMessage)
	app.ZmqSendMessage(topic, b)
}

func (app *TerraApp) HandleMintTx(ctx sdk.Context, data, txBytes []byte) {
	msgExecute := make(map[string]interface{})
	json.Unmarshal(data, &msgExecute)
	if msgExecute["open_position"] == nil {
		return
	}

	openPosition := msgExecute["open_position"].(map[string]interface{})
	collateralRatio, _ := strconv.ParseFloat(openPosition["collateral_ratio"].(string), 64)
	assetAddr := openPosition["asset_info"].(map[string]interface{})["token"].(map[string]interface{})["contract_addr"].(string)
	pairName := app.mirrorToken["reverse"][assetAddr]
	if pairName == "" {
		return
	}

	collateralAmount, _ := strconv.ParseFloat(openPosition["collateral"].(map[string]interface{})["amount"].(string), 64)
	collateralAmount /= collateralRatio
	collateralInfo := openPosition["collateral"].(map[string]interface{})["info"].(map[string]interface{})
	if collateralInfo["token"] != nil {
		tokenAddr := collateralInfo["token"].(map[string]interface{})["contract_addr"].(string)
		collateralAsset := app.terraToken["reverse"][tokenAddr]
		if collateralAsset != "AUST" {
			collateralAsset = app.mirrorToken["reverse"][tokenAddr]
			if collateralAsset == "" {
				return
			}

			oraclePrice := app.getMirrorOraclePrice(ctx, tokenAddr)
			collateralAmount *= oraclePrice
		} else {
			ancRate := app.getAncRate(ctx)
			collateralAmount *= ancRate
		}
	}

	oraclePrice := app.getMirrorOraclePrice(ctx, assetAddr)
	amount := collateralAmount / oraclePrice
	assetAmount := int(amount)

	zmqMessage := make(map[string]interface{})
	zmqMessage["data"] = make(map[string]interface{})
	zmqMessage["data"].(map[string]interface{})["pairName"] = pairName
	zmqMessage["data"].(map[string]interface{})["assetIn"] = pairName
	zmqMessage["data"].(map[string]interface{})["amount"] = assetAmount
	zmqMessage["data"].(map[string]interface{})["maxSpread"] = "1"
	zmqMessage["data"].(map[string]interface{})["price"] = "1"
	zmqMessage["hash"] = fmt.Sprintf("%X", tmhash.Sum(txBytes))
	zmqMessage["type"] = "mint"
	topic := "mirrorSwapStart"

	b, _ := msgpack.Marshal(zmqMessage)
	app.ZmqSendMessage(topic, b)
}

func (app *TerraApp) getAncRate(ctx sdk.Context) float64 {
	query := make(map[string]interface{})
	query["epoch_state"] = make(map[string]interface{})

	queryJson, _ := json.Marshal(query)

	q := wasmkeeper.NewWasmQuerier(app.WasmKeeper)
	result, _ := q.CustomQuery(ctx, app.GetWallets()["ancContract"], queryJson)
	jsonData := make(map[string]interface{})
	json.Unmarshal(result, &jsonData)
	ancRate, _ := strconv.ParseFloat(jsonData["exchange_rate"].(string), 64)
	return ancRate
}

func (app *TerraApp) getMirrorOraclePrice(ctx sdk.Context, address string) float64 {
	query := make(map[string]interface{})
	query["price"] = make(map[string]interface{})
	query["price"].(map[string]interface{})["base_asset"] = address
	query["price"].(map[string]interface{})["quote_asset"] = "uusd"
	queryJson, _ := json.Marshal(query)

	q := wasmkeeper.NewWasmQuerier(app.WasmKeeper)
	result, _ := q.CustomQuery(ctx, app.GetWallets()["mirrorOracle"], queryJson)
	jsonData := make(map[string]interface{})
	json.Unmarshal(result, &jsonData)
	oraclePrice, _ := strconv.ParseFloat(jsonData["rate"].(string), 64)
	return oraclePrice
}

func (app *TerraApp) SendMirrorBalances(ctx sdk.Context) {
	q := wasmkeeper.NewWasmQuerier(app.WasmKeeper)

	wallet := app.GetWallets()
	zmqMessage := make(map[string]interface{})
	query := make(map[string]interface{})

	walletAddr, _ := sdk.AccAddressFromBech32(wallet["mirrorWallet"])
	sequence, _ := app.AccountKeeper.GetSequence(ctx, walletAddr)
	ust := app.BankKeeper.GetBalance(ctx, walletAddr, "uusd")

	zmqMessage["balance"] = make(map[string]interface{})
	zmqMessage["sequence"] = sequence

	zmqMessage["balance"].(map[string]interface{})["UST"] = ust.Amount.String()
	luna := app.BankKeeper.GetBalance(ctx, walletAddr, "uluna")
	zmqMessage["balance"].(map[string]interface{})["LUNA"] = luna.Amount.String()

	query["balance"] = make(map[string]interface{})
	query["balance"].(map[string]interface{})["address"] = wallet["mirrorWallet"]

	queryJson, _ := json.Marshal(query)
	for assetName, contractAddress := range app.mirrorToken["normal"] {
		result, err := q.CustomQuery(ctx, contractAddress, queryJson)
		if err != nil {
			fmt.Println(err)
			continue
		}

		jsonData := make(map[string]interface{})
		json.Unmarshal(result, &jsonData)
		balance := jsonData["balance"]

		zmqMessage["balance"].(map[string]interface{})[assetName] = balance
	}

	result, err := q.CustomQuery(ctx, app.terraToken["normal"]["AUST"], queryJson)
	if err != nil {
		fmt.Println(err)
		return
	}

	jsonData := make(map[string]interface{})
	json.Unmarshal(result, &jsonData)
	balance := jsonData["balance"]

	zmqMessage["AUST"] = balance

	b, _ := msgpack.Marshal(zmqMessage)
	app.ZmqSendMessage("mirrorUpdateAccount", b)
}

func (app *TerraApp) SendAncRate(ctx sdk.Context) {
	ancRate := app.getAncRate(ctx)
	zmqMessage := make(map[string]interface{})
	zmqMessage["ancRate"] = ancRate

	b, _ := msgpack.Marshal(zmqMessage)
	app.ZmqSendMessage("ancRate", b)
}

func (app *TerraApp) SendTerraBalances(ctx sdk.Context) {

	wallet := app.GetWallets()
	zmqMessage := make(map[string]interface{})

	walletAddr, _ := sdk.AccAddressFromBech32(wallet["terraWallet"])
	sequence, _ := app.AccountKeeper.GetSequence(ctx, walletAddr)
	contractAddr, _ := sdk.AccAddressFromBech32(app.GetWallets()["terraContract"])
	ust := app.BankKeeper.GetBalance(ctx, contractAddr, "uusd")

	zmqMessage["balance"] = ust.Amount.String()
	zmqMessage["sequence"] = sequence

	b, _ := msgpack.Marshal(zmqMessage)
	app.ZmqSendMessage("terraUpdateAccount", b)
}

func (app *TerraApp) SendPools(ctx sdk.Context, types string) {
	fmt.Println(types, "Sendpools")

	pair := app.GetAddressMap(types + "Pair")
	token := app.GetAddressMap(types + "Token")

	zmqMessage := make(map[string]interface{})

	query := make(map[string]interface{})
	query["pool"] = make(map[string]interface{})

	queryJson, _ := json.Marshal(query)

	q := wasmkeeper.NewWasmQuerier(app.WasmKeeper)

	for name, contractAddress := range pair["normal"] {

		if name == "BLUNA-NLUNA" || name == "BETH-NETH" {
			continue
		}

		result, err := q.CustomQuery(ctx, contractAddress, queryJson)
		if err != nil {
			fmt.Println(err)
			continue
		}

		zmqMessage[name] = make(map[string]interface{})

		jsonData := make(map[string]([]map[string]interface{}))
		json.Unmarshal(result, &jsonData)
		assets := jsonData["assets"]
		assetName0 := extractAssetName(assets[0]["info"].(map[string]interface{}), token["reverse"])
		assetName1 := extractAssetName(assets[1]["info"].(map[string]interface{}), token["reverse"])

		if assetName0 == "" || assetName1 == "" {
			continue
		}

		amount0 := assets[0]["amount"]
		amount1 := assets[1]["amount"]
		zmqMessage[name].(map[string]interface{})[assetName0] = amount0
		zmqMessage[name].(map[string]interface{})[assetName1] = amount1
	}

	b, _ := msgpack.Marshal(zmqMessage)
	app.ZmqSendMessage(types+"UpdateReserve", b)
}

func extractAssetName(info map[string]interface{}, tokenReverse map[string]string) (assetName string) {
	if info["native_token"] != nil {
		denom := info["native_token"].(map[string]interface{})["denom"].(string)
		assetName = changeDenomName(denom)
	} else if info["token"] != nil {
		contract := info["token"].(map[string]interface{})["contract_addr"].(string)
		assetName = tokenReverse[contract]
	} else {
		assetName = ""
	}
	return assetName
}
