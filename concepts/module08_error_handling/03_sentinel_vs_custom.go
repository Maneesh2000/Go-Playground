// Module 08, Example 3 — Sentinel errors vs custom error types: a small,
// realistic "checkout" flow showing when each shines.
//
// Decision guide:
//   - Caller only needs to know WHICH condition happened -> sentinel value
//     (errors.New at package level, checked with errors.Is).
//   - Caller needs DATA about the failure (field name, limit, retry-after)
//     -> custom error type (struct with Error(), extracted with errors.As).
//
// Run with: go run 03_sentinel_vs_custom.go
package main

import (
	"errors"
	"fmt"
)

// ---- Sentinels: fixed, comparable conditions -----------------------------------
// Exported so callers can errors.Is against them. Once exported, they are
// part of your API forever — create them sparingly.
var (
	ErrCartEmpty   = errors.New("cart is empty")
	ErrOutOfStock  = errors.New("item out of stock")
	ErrPaymentDown = errors.New("payment provider unavailable") // retryable!
)

// ---- Custom error type: a condition that carries data ---------------------------
// "Insufficient funds" is only useful if the caller learns HOW MUCH is
// missing — perfect case for a typed error.
type InsufficientFundsError struct {
	Required  float64
	Available float64
}

func (e *InsufficientFundsError) Error() string {
	return fmt.Sprintf("insufficient funds: need %.2f, have %.2f (short %.2f)",
		e.Required, e.Available, e.Required-e.Available)
}

// ---- The domain logic --------------------------------------------------------------

type Cart struct {
	Items   []string
	Total   float64
	Balance float64
}

func checkout(c Cart) error {
	if len(c.Items) == 0 {
		return ErrCartEmpty // sentinel: nothing more to say
	}
	for _, item := range c.Items {
		if item == "unicorn" {
			// Wrap the sentinel to ADD context while staying detectable:
			return fmt.Errorf("item %q: %w", item, ErrOutOfStock)
		}
	}
	if c.Balance < c.Total {
		// Typed error: the caller may want the numbers.
		return &InsufficientFundsError{Required: c.Total, Available: c.Balance}
	}
	if c.Total > 10_000 {
		return fmt.Errorf("charging card: %w", ErrPaymentDown)
	}
	return nil
}

// placeOrder is the layer above; it wraps whatever checkout returns.
func placeOrder(c Cart) error {
	if err := checkout(c); err != nil {
		return fmt.Errorf("place order: %w", err)
	}
	fmt.Printf("order placed! %d items, %.2f charged\n", len(c.Items), c.Total)
	return nil
}

func main() {
	carts := []Cart{
		{}, // empty
		{Items: []string{"book", "unicorn"}, Total: 60, Balance: 100},     // out of stock
		{Items: []string{"laptop"}, Total: 1500, Balance: 900},            // broke
		{Items: []string{"castle"}, Total: 50_000, Balance: 99_999},       // payment down
		{Items: []string{"coffee", "keyboard"}, Total: 140, Balance: 200}, // fine
	}

	for i, cart := range carts {
		fmt.Printf("--- cart %d ---\n", i+1)
		err := placeOrder(cart)
		if err == nil {
			continue
		}

		// The caller branches on the KIND of failure. Note: always Is/As,
		// never string-matching on err.Error() — messages are for humans,
		// identities and types are for programs.
		switch {
		case errors.Is(err, ErrCartEmpty):
			fmt.Println("hint: add something to your cart first")

		case errors.Is(err, ErrOutOfStock):
			fmt.Println("hint: remove the unavailable item; full error:", err)

		case errors.Is(err, ErrPaymentDown):
			fmt.Println("hint: transient failure — safe to RETRY later")

		default:
			// Typed errors: extract the data with errors.As.
			var ife *InsufficientFundsError
			if errors.As(err, &ife) {
				fmt.Printf("hint: top up %.2f to complete this purchase\n",
					ife.Required-ife.Available)
				break
			}
			fmt.Println("unexpected error:", err)
		}
	}
}
