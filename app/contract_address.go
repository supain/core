package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type Contract struct {
	AssetName string `json:"assetName"`
	Address   string `json:"address"`
}

type Contracts struct {
	Contracts []Contract `json:"contracts"`
}

type Wallet struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type Wallets struct {
	Addresses []Wallet `json:"wallets"`
}

func LoadWallet() map[string]string {
	userHomeDir, _ := os.UserHomeDir()
	data, err := os.Open(userHomeDir + "/workspace/arbitrage-config/wallet.json")
	if err != nil {
		fmt.Println(err)
	}
	defer data.Close()

	byteValue, _ := ioutil.ReadAll(data)
	var wallets Wallets
	json.Unmarshal(byteValue, &wallets)
	walletMap := make(map[string]string)
	for _, wallet := range wallets.Addresses {
		walletMap[wallet.Name] = wallet.Address
	}
	return walletMap
}

func (app *TerraApp) GetWallets() map[string]string {
	return app.walletMap
}

func LoadConfig(fileName string) map[string]map[string]string {
	userHomeDir, _ := os.UserHomeDir()
	data, err := os.Open(userHomeDir + "/workspace/arbitrage-config/" + fileName)

	if err != nil {
		fmt.Println(err)
	}
	defer data.Close()
	byteValue, _ := ioutil.ReadAll(data)
	var contracts Contracts
	json.Unmarshal(byteValue, &contracts)
	contractMap := make(map[string]map[string]string)
	normalMap := make(map[string]string)
	reverseMap := make(map[string]string)
	for _, contract := range contracts.Contracts {
		normalMap[contract.AssetName] = contract.Address
		reverseMap[contract.Address] = contract.AssetName
	}
	contractMap["normal"] = normalMap
	contractMap["reverse"] = reverseMap
	return contractMap
}

func (app *TerraApp) GetAddressMap(name string) map[string]map[string]string {
	if name == "mirrorPair" {
		return app.mirrorPair
	} else if name == "mirrorToken" {
		return app.mirrorToken
	} else if name == "terraPair" {
		return app.terraPair
	} else if name == "terraToken" {
		return app.terraToken
	} else {
		return nil
	}
}
