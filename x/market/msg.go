package market

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RouterKey is they name of the market module
const RouterKey = "market"

//--------------------------------------------------------
//--------------------------------------------------------

// SwapMsg contains a swap request
type SwapMsg struct {
	Trader    sdk.AccAddress // Address of the trader
	OfferCoin sdk.Coin       // Coin being offered
	AskDenom  string         // Denom of the coin to swap to
}

// NewSwapMsg creates a SwapMsg instance
func NewSwapMsg(traderAddress sdk.AccAddress, offerCoin sdk.Coin, askCoin string) SwapMsg {
	return SwapMsg{
		Trader:    traderAddress,
		OfferCoin: offerCoin,
		AskDenom:  askCoin,
	}
}

// Route Implements Msg
func (msg SwapMsg) Route() string { return "market" }

// Type implements sdk.Msg
func (msg SwapMsg) Type() string { return "swap" }

// GetSignBytes Implements Msg
func (msg SwapMsg) GetSignBytes() []byte {
	b, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}
	return b
}

// GetSigners Implements Msg
func (msg SwapMsg) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.Trader}
}

// ValidateBasic Implements Msg
func (msg SwapMsg) ValidateBasic() sdk.Error {
	if len(msg.Trader) == 0 {
		return sdk.ErrInvalidAddress("Invalid address: " + msg.Trader.String())
	}

	if msg.OfferCoin.Amount.LT(sdk.ZeroInt()) {
		return ErrInsufficientSwapCoins(DefaultCodespace, msg.OfferCoin.Amount)
	}

	if msg.OfferCoin.Denom == msg.AskDenom {
		return ErrRecursiveSwap(DefaultCodespace, msg.AskDenom)
	}

	return nil
}

// String Implements Msg
func (msg SwapMsg) String() string {
	return fmt.Sprintf("SwapMsg{trader %v, offer %v, ask %s}", msg.Trader, msg.OfferCoin, msg.AskDenom)
}

type SwapHistory []SwapMsg

// String implements fmt.Stringer interface
func (swaps SwapHistory) String() (out string) {
	for _, swap := range swaps {
		out += fmt.Sprintf("\n  %s", swap.String())
	}
	return
}