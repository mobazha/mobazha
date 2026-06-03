package notifier

import (
	"fmt"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/events"
)

// formatTelegramEvent converts an event into a Telegram Markdown message.
func formatTelegramEvent(meta events.EventMeta, event interface{}) string {
	icon := categoryIcon(meta.Category)
	title := eventTitle(meta.Name)

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s *%s*\n", icon, escapeTelegramMarkdown(title))

	switch e := event.(type) {
	case *events.NewOrder:
		fmt.Fprintf(&sb, "Order: `%s`\n", e.OrderID)
		if e.BuyerName != "" {
			fmt.Fprintf(&sb, "Buyer: %s\n", escapeTelegramMarkdown(e.BuyerName))
		}
		if e.Title != "" {
			fmt.Fprintf(&sb, "Item: %s\n", escapeTelegramMarkdown(e.Title))
		}

	case *events.OrderFunded:
		fmt.Fprintf(&sb, "Order: `%s`\n", e.OrderID)

	case *events.OrderPaymentReceived:
		fmt.Fprintf(&sb, "Order: `%s`\n", e.OrderID)

	case *events.OrderConfirmation:
		fmt.Fprintf(&sb, "Order: `%s`\n", e.OrderID)

	case *events.OrderShipment:
		fmt.Fprintf(&sb, "Order: `%s`\n", e.OrderID)

	case *events.OrderCompletion:
		fmt.Fprintf(&sb, "Order: `%s`\n", e.OrderID)

	case *events.OrderRated:
		fmt.Fprintf(&sb, "Order: `%s`\n", e.OrderID)

	case *events.OrderCancel:
		fmt.Fprintf(&sb, "Order: `%s`\n", e.OrderID)

	case *events.OrderDeclined:
		fmt.Fprintf(&sb, "Order: `%s`\n", e.OrderID)

	case *events.Refund:
		fmt.Fprintf(&sb, "Order: `%s`\n", e.OrderID)

	case *events.DisputeOpen:
		fmt.Fprintf(&sb, "Order: `%s`\n", e.OrderID)
		if e.DisputerName != "" {
			fmt.Fprintf(&sb, "Disputer: %s\n", escapeTelegramMarkdown(e.DisputerName))
		}

	case *events.DisputeClose:
		fmt.Fprintf(&sb, "Order: `%s`\n", e.OrderID)

	default:
		fmt.Fprintf(&sb, "Event: `%s`\n", meta.Name)
	}

	return sb.String()
}

func categoryIcon(category string) string {
	switch category {
	case "order":
		return "🛒"
	case "dispute":
		return "⚠️"
	case "chat":
		return "💬"
	case "wallet":
		return "💰"
	case "social":
		return "👤"
	case "publish":
		return "📦"
	default:
		return "🔔"
	}
}

func eventTitle(name string) string {
	titles := map[string]string{
		"order.created":          "New Order",
		"order.funded":           "Order Funded",
		"order.payment_received": "Payment Received",
		"order.confirmed":        "Order Confirmed",
		"order.shipped":          "Order Shipped",
		"order.completed":        "Order Completed",
		"order.rated":            "Order Rated",
		"order.cancelled":        "Order Cancelled",
		"order.declined":         "Order Declined",
		"order.refunded":         "Order Refunded",
		"order.vendor_finalized": "Vendor Finalized",
		"dispute.opened":         "Dispute Opened",
		"dispute.closed":         "Dispute Closed",
		"dispute.accepted":       "Dispute Accepted",
		"dispute.case_open":      "Case Opened",
		"dispute.case_update":    "Case Updated",
		"chat.message":           "New Message",
		"chat.read":              "Message Read",
		"wallet.tx_received":     "Transaction Received",
		"wallet.block_received":  "Block Received",
		"social.follow":          "New Follower",
		"social.unfollow":        "Unfollowed",
	}
	if t, ok := titles[name]; ok {
		return t
	}
	return strings.ReplaceAll(name, ".", " → ")
}

func escapeTelegramMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"*", "\\*",
		"_", "\\_",
		"`", "\\`",
		"[", "\\[",
	)
	return replacer.Replace(s)
}
