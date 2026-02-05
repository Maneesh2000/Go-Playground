// Module 07, Example 4 — A plug-in style Notifier: interfaces as extension
// points.
//
// The scenario: an app needs to send notifications. Today via email and SMS;
// tomorrow maybe Slack, push, carrier pigeon. The core app should NOT change
// when a new channel is added — new channels just "plug in" by satisfying
// a small interface.
//
// Run with: go run 04_notifier_plugin.go
package main

import (
	"fmt"
	"strings"
)

// Notifier is the plug-in contract. ONE method. Anything that can deliver a
// message qualifies. The core app depends only on this.
type Notifier interface {
	Notify(message string) error
}

// ---- Plug-in 1: email ---------------------------------------------------------

type EmailNotifier struct {
	Address string
}

func (e EmailNotifier) Notify(message string) error {
	if !strings.Contains(e.Address, "@") {
		return fmt.Errorf("invalid email address %q", e.Address)
	}
	fmt.Printf("  [email] to %-20s : %s\n", e.Address, message)
	return nil
}

// ---- Plug-in 2: SMS -------------------------------------------------------------

type SMSNotifier struct {
	Number string
}

func (s SMSNotifier) Notify(message string) error {
	// SMS is short — truncate long messages.
	if len(message) > 20 {
		message = message[:17] + "..."
	}
	fmt.Printf("  [sms]   to %-20s : %s\n", s.Number, message)
	return nil
}

// ---- Plug-in 3: Slack — added later WITHOUT touching the core --------------------

type SlackNotifier struct {
	Channel string
}

func (s SlackNotifier) Notify(message string) error {
	fmt.Printf("  [slack] to %-20s : %s\n", "#"+s.Channel, message)
	return nil
}

// ---- The core app ------------------------------------------------------------------

// AlertService is the "application". Note it stores Notifiers — it has no
// idea (and doesn't care) whether they're email, SMS, or something invented
// next year. This is dependency injection, Go style.
type AlertService struct {
	notifiers []Notifier
}

// Register plugs a new channel in at runtime.
func (a *AlertService) Register(n Notifier) {
	a.notifiers = append(a.notifiers, n)
}

// Broadcast fans one message out to every registered channel, collecting
// failures instead of stopping at the first one.
func (a *AlertService) Broadcast(message string) {
	fmt.Printf("broadcasting: %q\n", message)
	for _, n := range a.notifiers {
		if err := n.Notify(message); err != nil {
			fmt.Printf("  [error] %T failed: %v\n", n, err)
		}
	}
}

func main() {
	svc := &AlertService{}

	// Wire up channels. In a real app these might come from a config file.
	svc.Register(EmailNotifier{Address: "ops@example.com"})
	svc.Register(SMSNotifier{Number: "+1-555-0100"})
	svc.Register(SlackNotifier{Channel: "alerts"})
	// A broken one, to show errors don't stop the others:
	svc.Register(EmailNotifier{Address: "not-an-email"})

	svc.Broadcast("Deployment finished successfully, version 2.4.1 is live")

	fmt.Println()
	svc.Broadcast("Disk 80% full")

	// Takeaways from this pattern:
	//  1. The interface lives with the CONSUMER (AlertService defines what
	//     it needs), not with the implementations.
	//  2. Adding SlackNotifier required ZERO changes to AlertService.
	//  3. Testing is easy: a fake Notifier that records messages satisfies
	//     the same interface (that's how mocks work in Go — no framework
	//     needed).
}
