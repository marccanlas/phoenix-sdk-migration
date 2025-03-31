package main

import (
	"errors"
	"fmt"
)

type MarketState struct{}

type ClockData struct {
	Slot          int64
	UnixTimestamp int64
}

type LadderLevel struct {
	PriceInTicks   float64
	SizeInBaseLots float64
}

type UiLadderLevel struct {
	Price    float64
	Quantity float64
}

type UiLadder struct {
	Asks []UiLadderLevel
	Bids []UiLadderLevel
}

type Hoenix struct {
	MarketStates map[string]MarketState
	Clock        ClockData
	Data         struct {
		Bids map[string]struct {
			LastValidSlot                   int64
			LastValidUnixTimestampInSeconds int64
			NumBaseLots                     float64
			PriceInTicks                    float64
		}
		Asks map[string]struct {
			LastValidSlot                   int64
			LastValidUnixTimestampInSeconds int64
			NumBaseLots                     float64
			PriceInTicks                    float64
		}
		Header struct {
			BaseParams              struct{ Decimals int }
			QuoteParams             struct{ Decimals int }
			RawBaseUnitsPerBaseUnit float64
		}
		TakerFeeBps float64
	}
}

const (
	FeeScale = 10_000
)

type Side int

const (
	Bid Side = iota
	Ask
)

type QuoteParams struct {
	InAmount float64
	AToB     bool
}

type Quote struct {
	InAmount  float64
	OutAmount float64
}

// GetQuote now returns the updated ladder instead of liquidity
func (h *Hoenix) GetQuote(params QuoteParams, ladder *UiLadder) (*Quote, *UiLadder, error) {
	side := Bid
	if !params.AToB {
		side = Ask
	}
	expectedOutAmount, err := h.getExpectedOutAmount(ladder, side, h.Data.TakerFeeBps, params.InAmount)
	if err != nil {
		if err.Error() == "not enough liquidity to fulfill the trade" {
			return nil, nil, errors.New("not enough liquidity for the requested amount")
		}
		return nil, nil, err
	}

	// Instead of using liquidity, we will update the ladder directly
	if params.AToB {
		h.updateLadderLiquidity(ladder, Ask, expectedOutAmount) // Updates the asks ladder
	} else {
		h.updateLadderLiquidity(ladder, Bid, params.InAmount) // Updates the bids ladder
	}

	// Check if the ladder has sufficient liquidity
	if len(ladder.Asks) == 0 || len(ladder.Bids) == 0 {
		return nil, nil, errors.New("updated ladder has no more asks or bids")
	}

	// Return the Quote and updated ladder instead of liquidity
	return &Quote{
		InAmount:  params.InAmount,
		OutAmount: expectedOutAmount,
	}, ladder, nil
}

func (h *Hoenix) getExpectedOutAmount(uiLadder *UiLadder, side Side, takerFeeBps float64, inAmount float64) (float64, error) {
	fmt.Printf("Ladder: %+v\n", uiLadder)
	if inAmount <= 0 {
		return 0, errors.New("input amount must be greater than zero")
	}

	adjustedAmount := h.applyTakerFee(inAmount, takerFeeBps)
	if side == Bid {
		return h.getBaseUnitsOutFromQuoteUnitsIn(uiLadder.Asks, adjustedAmount)
	} else {
		return h.getQuoteUnitsOutFromBaseUnitsIn(uiLadder.Bids, adjustedAmount)
	}
}

func (h *Hoenix) applyTakerFee(amount, takerFeeBps float64) float64 {
	return amount / (1 + takerFeeBps/FeeScale)
}

func (h *Hoenix) getBaseUnitsOutFromQuoteUnitsIn(asks []UiLadderLevel, quoteUnitsIn float64) (float64, error) {
	if quoteUnitsIn <= 0 {
		return 0, errors.New("quote units must be greater than zero")
	}
	return h.calculateBaseAmountFromQuoteBudget(asks, quoteUnitsIn)
}

func (h *Hoenix) getQuoteUnitsOutFromBaseUnitsIn(bids []UiLadderLevel, baseUnitsIn float64) (float64, error) {
	if baseUnitsIn <= 0 {
		return 0, errors.New("base units must be greater than zero")
	}
	return h.calculateQuoteAmountFromBaseBudget(bids, baseUnitsIn)
}

func (h *Hoenix) calculateBaseAmountFromQuoteBudget(asks []UiLadderLevel, quoteBudget float64) (float64, error) {
	baseAmount := 0.0
	for _, level := range asks {
		if level.Price*level.Quantity >= quoteBudget {
			baseAmount += quoteBudget / level.Price
			quoteBudget = 0
			break
		}
		baseAmount += level.Quantity
		quoteBudget -= level.Price * level.Quantity
		if quoteBudget <= 0 {
			break
		}
	}

	if quoteBudget > 0 {
		return baseAmount, errors.New("not enough liquidity to fulfill the trade")
	}
	fmt.Printf("baseAmount==> %+v\n", baseAmount)
	return baseAmount, nil
}

func (h *Hoenix) calculateQuoteAmountFromBaseBudget(bids []UiLadderLevel, baseBudget float64) (float64, error) {
	quoteAmount := 0.0
	for _, level := range bids {
		if level.Quantity >= baseBudget {
			quoteAmount += baseBudget * level.Price
			baseBudget = 0
			break
		}
		quoteAmount += level.Quantity * level.Price
		baseBudget -= level.Quantity
		if baseBudget <= 0 {
			break
		}
	}

	if baseBudget > 0 {
		return quoteAmount, errors.New("not enough liquidity to fulfill the trade")
	}
	fmt.Printf("quoteAmount==> %+v\n", quoteAmount)
	return quoteAmount, nil
}

func (h *Hoenix) updateLadderLiquidity(ladder *UiLadder, side Side, amount float64) {
	if side == Bid {
		for i := range ladder.Bids {
			if ladder.Bids[i].Quantity >= amount {
				ladder.Bids[i].Quantity -= amount
				break
			} else {
				amount -= ladder.Bids[i].Quantity
				ladder.Bids[i].Quantity = 0
			}
		}
	} else {
		for i := range ladder.Asks {
			if ladder.Asks[i].Quantity >= amount {
				ladder.Asks[i].Quantity -= amount
				break
			} else {
				amount -= ladder.Asks[i].Quantity
				ladder.Asks[i].Quantity = 0
			}
		}
	}
}

func main() {
	hoenix := &Hoenix{}
	hoenix.Data.TakerFeeBps = 5

	ladder := UiLadder{
		Bids: []UiLadderLevel{
			{Price: 20, Quantity: 10},
			{Price: 15, Quantity: 5},
			{Price: 10, Quantity: 2},
		},
		Asks: []UiLadderLevel{
			{Price: 25, Quantity: 10},
			{Price: 30, Quantity: 5},
			{Price: 35, Quantity: 2},
		},
	}

	quoteParams1 := QuoteParams{
		InAmount: 150, // Buy x SOL
		AToB:     true,
	}

	q1, updatedLadder1, err := hoenix.GetQuote(quoteParams1, &ladder)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Quote 1 (x SOL buy): %+v\n", q1)
	fmt.Printf("Updated Ladder after x SOL buy: %+v\n", updatedLadder1)

	quoteParams2 := QuoteParams{
		InAmount: 50, // Buy y SOL
		AToB:     true,
	}
	q2, updatedLadder2, err := hoenix.GetQuote(quoteParams2, &ladder)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Quote 2 (y SOL buy): %+v\n", q2)
	fmt.Printf("Updated Ladder after y SOL buy: %+v\n", updatedLadder2)
}
