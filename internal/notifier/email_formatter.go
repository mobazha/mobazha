package notifier

import (
	"fmt"
	"html"
	"strings"

	"github.com/mobazha/mobazha/pkg/events"
)

// formatEmailEvent produces a two-part string: first line is the subject,
// remainder is an HTML body. The caller (EmailSender.Send) splits on "\n".
// storeURL is the store's external base URL for generating action links.
func formatEmailEvent(meta events.EventMeta, event interface{}, storeURL string) string {
	title := eventTitle(meta.Name)
	icon := categoryIcon(meta.Category)

	var subject string
	var rows []emailRow
	var actionURL string

	switch e := event.(type) {
	case *events.NewOrder:
		subject = fmt.Sprintf("New Order: %s", e.Title)
		rows = append(rows,
			emailRow{"Order ID", e.OrderID},
			emailRow{"Item", e.Title},
		)
		if e.BuyerName != "" {
			rows = append(rows, emailRow{"Buyer", e.BuyerName})
		}
		if e.Price.Amount != "" {
			rows = append(rows, emailRow{"Price", e.Price.Amount + " " + e.Price.CurrencyCode})
		}
		actionURL = orderActionURL(storeURL, e.OrderID)

	case *events.OrderFunded:
		subject = fmt.Sprintf("Payment Received: %s", e.Title)
		rows = append(rows,
			emailRow{"Order ID", e.OrderID},
			emailRow{"Item", e.Title},
		)
		if e.Price.Amount != "" {
			rows = append(rows, emailRow{"Amount", e.Price.Amount + " " + e.Price.CurrencyCode})
		}
		actionURL = orderActionURL(storeURL, e.OrderID)

	case *events.OrderPaymentReceived:
		subject = fmt.Sprintf("Payment Update for Order %s", truncateID(e.OrderID))
		rows = append(rows, emailRow{"Order ID", e.OrderID})
		if e.FundingTotal != "" {
			rows = append(rows, emailRow{"Amount", e.FundingTotal + " " + e.CoinType})
		}
		actionURL = orderActionURL(storeURL, e.OrderID)

	case *events.OrderConfirmation:
		subject = fmt.Sprintf("Order Confirmed: %s", truncateID(e.OrderID))
		rows = append(rows, emailRow{"Order ID", e.OrderID})
		actionURL = orderActionURL(storeURL, e.OrderID)

	case *events.OrderShipment:
		subject = fmt.Sprintf("Order Shipped: %s", truncateID(e.OrderID))
		rows = append(rows, emailRow{"Order ID", e.OrderID})
		actionURL = orderActionURL(storeURL, e.OrderID)

	case *events.OrderCompletion:
		subject = fmt.Sprintf("Order Completed: %s", truncateID(e.OrderID))
		rows = append(rows, emailRow{"Order ID", e.OrderID})
		actionURL = orderActionURL(storeURL, e.OrderID)

	case *events.OrderRated:
		subject = fmt.Sprintf("Order Rated: %s", truncateID(e.OrderID))
		rows = append(rows, emailRow{"Order ID", e.OrderID})
		actionURL = orderActionURL(storeURL, e.OrderID)

	case *events.OrderCancel:
		subject = fmt.Sprintf("Order Cancelled: %s", truncateID(e.OrderID))
		rows = append(rows, emailRow{"Order ID", e.OrderID})
		actionURL = orderActionURL(storeURL, e.OrderID)

	case *events.OrderDeclined:
		subject = fmt.Sprintf("Order Declined: %s", truncateID(e.OrderID))
		rows = append(rows, emailRow{"Order ID", e.OrderID})
		actionURL = orderActionURL(storeURL, e.OrderID)

	case *events.Refund:
		subject = fmt.Sprintf("Refund Issued: %s", truncateID(e.OrderID))
		rows = append(rows, emailRow{"Order ID", e.OrderID})
		actionURL = orderActionURL(storeURL, e.OrderID)

	case *events.DisputeOpen:
		subject = fmt.Sprintf("Dispute Opened: %s", truncateID(e.OrderID))
		rows = append(rows, emailRow{"Order ID", e.OrderID})
		if e.DisputerName != "" {
			rows = append(rows, emailRow{"Opened by", e.DisputerName})
		}
		actionURL = orderActionURL(storeURL, e.OrderID)

	case *events.DisputeClose:
		subject = fmt.Sprintf("Dispute Closed: %s", truncateID(e.OrderID))
		rows = append(rows, emailRow{"Order ID", e.OrderID})
		actionURL = orderActionURL(storeURL, e.OrderID)

	default:
		subject = title
		rows = append(rows, emailRow{"Event", meta.Name})
	}

	body := renderEmailHTML(icon, title, rows, actionURL)
	return subject + "\n" + body
}

func orderActionURL(storeURL, orderID string) string {
	if storeURL == "" || orderID == "" {
		return ""
	}
	return storeURL + "/admin/orders/" + orderID
}

type emailRow struct {
	Label string
	Value string
}

func renderEmailHTML(icon, title string, rows []emailRow, actionURL string) string {
	var sb strings.Builder

	sb.WriteString(`<html><body style="margin:0;padding:0;background-color:#f4f4f5">`)
	sb.WriteString(`<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;max-width:600px;margin:0 auto;padding:24px">`)

	// Header
	sb.WriteString(`<div style="background:#00BCD4;color:#fff;padding:20px 24px;border-radius:8px 8px 0 0">`)
	sb.WriteString(fmt.Sprintf(`<h1 style="margin:0;font-size:20px;font-weight:600">%s %s</h1>`, icon, html.EscapeString(title)))
	sb.WriteString(`</div>`)

	// Body
	sb.WriteString(`<div style="background:#fff;padding:24px;border:1px solid #e4e4e7;border-top:none;border-radius:0 0 8px 8px">`)

	if len(rows) > 0 {
		sb.WriteString(`<table style="width:100%;border-collapse:collapse">`)
		for _, r := range rows {
			sb.WriteString(`<tr>`)
			sb.WriteString(fmt.Sprintf(`<td style="padding:8px 0;color:#71717a;font-size:14px;vertical-align:top;width:120px">%s</td>`, html.EscapeString(r.Label)))
			sb.WriteString(fmt.Sprintf(`<td style="padding:8px 0;color:#18181b;font-size:14px;word-break:break-all">%s</td>`, html.EscapeString(r.Value)))
			sb.WriteString(`</tr>`)
		}
		sb.WriteString(`</table>`)
	}

	if actionURL != "" {
		sb.WriteString(`<div style="margin-top:20px;text-align:center">`)
		sb.WriteString(fmt.Sprintf(
			`<a href="%s" style="display:inline-block;padding:12px 28px;background:#00BCD4;color:#fff;text-decoration:none;border-radius:6px;font-size:14px;font-weight:600">View Details</a>`,
			html.EscapeString(actionURL),
		))
		sb.WriteString(`</div>`)
	}

	sb.WriteString(`<div style="margin-top:20px;padding-top:16px;border-top:1px solid #e4e4e7;text-align:center">`)
	sb.WriteString(`<p style="color:#a1a1aa;font-size:12px;margin:0">This notification was sent by your Mobazha store.</p>`)
	sb.WriteString(`</div>`)

	sb.WriteString(`</div>`) // body card
	sb.WriteString(`</div>`) // wrapper
	sb.WriteString(`</body></html>`)

	return sb.String()
}

func truncateID(id string) string {
	if len(id) > 12 {
		return id[:12] + "..."
	}
	return id
}
