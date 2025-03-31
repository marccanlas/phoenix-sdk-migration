package main

import (
	"fmt"
	"math"
)

type LifinityLiquidity struct {
	A  uint64  // Reserve for base token (e.g., SOL)
	B  uint64  // Reserve for quote token (e.g., USDC)
	_k float64 // Constant product (x * y = k)
}

func NewLifinityLiquidity(a, b uint64) *LifinityLiquidity {
	return &LifinityLiquidity{
		A:  a,
		B:  b,
		_k: float64(a) * float64(b), // Constant product
	}
}

func (l *LifinityLiquidity) Price(aToB bool) float64 {
	if aToB {
		return float64(l.A) / float64(l.B)
	} else {
		return float64(l.B) / float64(l.A)
	}
}

func (l *LifinityLiquidity) K() float64 {
	return l._k
}

const (
	LifinityFeeRate = 50 // e.g., 50 BPS = 0.5%
)

type QuoteParams struct {
	InAmount uint64 // Input token amount for the swap
	AToB     bool   // Direction: true for base to quote (A -> B), false for quote to base (B -> A)
}

type Quote struct {
	InAmount      uint64 // Amount of input tokens
	OutAmount     uint64 // Amount of output tokens
	PriceImpactBP uint   // Price impact in basis points
}

func (l *LifinityLiquidity) GetQuote(params QuoteParams) (*Quote, error) {
	feeAmount := params.InAmount * LifinityFeeRate / 10_000

	var outAmount uint64
	var afterA, afterB uint64

	if params.AToB {
		// A to B swap (Base -> Quote)
		afterA = l.A + params.InAmount - feeAmount
		afterB = uint64(l.K() / float64(afterA)) // Calculate B based on new A
		outAmount = l.B - afterB - 1             // Subtract 1 to account for precision loss
	} else {
		// B to A swap (Quote -> Base)
		afterB = l.B + params.InAmount - feeAmount
		afterA = uint64(l.K() / float64(afterB)) // Calculate A based on new B
		outAmount = l.A - afterA - 1             // Subtract 1 for precision
	}

	if afterA == 0 || afterB == 0 {
		return nil, fmt.Errorf("afterLiquidity is zero")
	}

	l.A = afterA
	l.B = afterB

	beforePrice := l.Price(params.AToB)
	afterPrice := float64(afterA) / float64(afterB)
	priceImpactBP := math.Abs(afterPrice-beforePrice) / beforePrice * 10_000

	return &Quote{
		InAmount:      params.InAmount,
		OutAmount:     outAmount,
		PriceImpactBP: uint(priceImpactBP),
	}, nil
}

func main() {
	liquidity := NewLifinityLiquidity(1000, 20000)

	// First swap (SOL to USDC)
	params := QuoteParams{
		InAmount: 10,   // Input 10 SOL
		AToB:     true, // Swap from SOL to USDC
	}

	quote, err := liquidity.GetQuote(params)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Quote 1: InAmount=%d, OutAmount=%d, PriceImpactBP=%d\n", quote.InAmount, quote.OutAmount, quote.PriceImpactBP)
	fmt.Printf("Updated Liquidity after 1st swap: A=%d, B=%d\n", liquidity.A, liquidity.B)

	// Second swap (USDC to SOL), now based on the updated liquidity
	params1 := QuoteParams{
		InAmount: 500,   // Input 500 USDC
		AToB:     false, // Swap from USDC to SOL
	}

	quote1, err := liquidity.GetQuote(params1)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Quote 2: InAmount=%d, OutAmount=%d, PriceImpactBP=%d\n", quote1.InAmount, quote1.OutAmount, quote1.PriceImpactBP)
	fmt.Printf("Updated Liquidity after 2nd swap: A=%d, B=%d\n", liquidity.A, liquidity.B)
}
