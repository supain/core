package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (querier WasmQuerier) CustomQuery(ctx sdk.Context, contractAddress string, queryMsg []byte) (result []byte, err error) {
	// external query gas limit must be specified here
	ctx = ctx.WithGasMeter(sdk.NewGasMeter(querier.keeper.wasmConfig.ContractQueryGasLimit))

	var contractAddr sdk.AccAddress
	contractAddr, err = sdk.AccAddressFromBech32(contractAddress)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	result, err = querier.keeper.queryToContract(ctx, contractAddr, queryMsg)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return result, nil
}
