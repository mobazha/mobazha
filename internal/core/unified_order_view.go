package core

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

// standardOrderFetcher is a narrow interface covering the GetSales method of
// OrderAppService.  We define it locally so UnifiedOrderView does not depend
// on the full contracts.OrderService (which OrderAppService may not fully
// implement).
type standardOrderFetcher interface {
	GetSales(stateFilter []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Order, int64, error)
}

// UnifiedOrderView merges standard escrow-based orders and guest checkout
// orders into a single normalised list for the seller dashboard.
type UnifiedOrderView struct {
	orderSvc standardOrderFetcher
	guestSvc contracts.GuestOrderService
	db       database.Database
}

func NewUnifiedOrderView(orderSvc standardOrderFetcher, guestSvc contracts.GuestOrderService, db database.Database) *UnifiedOrderView {
	return &UnifiedOrderView{orderSvc: orderSvc, guestSvc: guestSvc, db: db}
}

// ListOrders returns a merged, sorted, and paginated list of orders.
func (v *UnifiedOrderView) ListOrders(ctx context.Context, f contracts.OrderListFilter) ([]contracts.OrderSummary, *contracts.OrderListMeta, error) {
	if f.PageSize <= 0 {
		f.PageSize = 20
	}
	if f.Page < 0 {
		f.Page = 0
	}

	var combined []contracts.OrderSummary
	var totalStandard, totalGuest int64

	if f.View == "" || f.View == "all" || f.View == "standard" {
		stds, err := v.fetchStandard(f)
		if err != nil {
			return nil, nil, err
		}
		combined = append(combined, stds...)
		totalStandard = int64(len(stds))
	}

	if f.View == "" || f.View == "all" || f.View == "guest" {
		guests, total, err := v.fetchGuest(ctx, f)
		if err != nil {
			return nil, nil, err
		}
		combined = append(combined, guests...)
		totalGuest = total
	}

	sort.Slice(combined, func(i, j int) bool {
		return combined[i].CreatedAt.After(combined[j].CreatedAt)
	})

	total := totalStandard + totalGuest

	start := f.Page * f.PageSize
	if start > len(combined) {
		start = len(combined)
	}
	end := start + f.PageSize
	if end > len(combined) {
		end = len(combined)
	}

	return combined[start:end], &contracts.OrderListMeta{
		Total:    total,
		Page:     f.Page,
		PageSize: f.PageSize,
	}, nil
}

func (v *UnifiedOrderView) fetchStandard(f contracts.OrderListFilter) ([]contracts.OrderSummary, error) {
	if v.orderSvc == nil {
		return nil, nil
	}

	orders, _, err := v.orderSvc.GetSales(nil, "", false, false, -1, nil)
	if err != nil {
		return nil, err
	}

	out := make([]contracts.OrderSummary, 0, len(orders))
	for _, order := range orders {
		s := convertStandardOrder(order)
		if s == nil {
			continue
		}
		if f.State != "" && s.State != f.State {
			continue
		}
		out = append(out, *s)
	}
	return out, nil
}

func convertStandardOrder(order models.Order) *contracts.OrderSummary {
	oo, err := order.OrderOpenMessage()
	if err != nil {
		return nil
	}

	var title string
	if len(oo.Listings) > 0 && oo.Listings[0] != nil && oo.Listings[0].Listing != nil {
		title = oo.Listings[0].Listing.Item.Title
	}

	var items []contracts.ItemBrief
	for _, item := range oo.Items {
		q, _ := strconv.Atoi(item.Quantity)
		if q <= 0 {
			q = 1
		}
		t := title
		if item.ListingHash != "" && len(oo.Listings) > 1 {
			t = findListingTitle(oo.Listings, item.ListingHash)
		}
		items = append(items, contracts.ItemBrief{Title: t, Quantity: q})
	}
	if len(items) == 0 {
		items = []contracts.ItemBrief{{Title: title, Quantity: 1}}
	}

	buyerName := "Buyer"
	if oo.BuyerID != nil {
		if oo.BuyerID.Name != "" {
			buyerName = oo.BuyerID.Name
		} else if oo.BuyerID.Handle != "" {
			buyerName = oo.BuyerID.Handle
		}
	}

	paymentCoin := ""
	if ps, pErr := order.PaymentSentMessage(); pErr == nil {
		paymentCoin = ps.Coin
	}

	priceSummary := contracts.PriceSummary{
		Amount:       oo.Amount,
		CurrencyCode: oo.PricingCoin,
	}
	if pcDef, lookupErr := models.CurrencyDefinitions.Lookup(oo.PricingCoin); lookupErr == nil && pcDef != nil {
		priceSummary.CurrencyCode = string(pcDef.Code)
		priceSummary.Divisibility = uint32(pcDef.Divisibility)
	}

	ts := time.Now()
	if oo.Timestamp != nil {
		ts = oo.Timestamp.AsTime()
	}

	return &contracts.OrderSummary{
		ID:          order.ID.String(),
		Type:        "standard",
		State:       order.State.String(),
		BuyerName:   buyerName,
		Items:       items,
		Total:       priceSummary,
		PaymentCoin: paymentCoin,
		CreatedAt:   ts,
		UpdatedAt:   ts,
	}
}

func findListingTitle(listings []*pb.SignedListing, hash string) string {
	for _, sl := range listings {
		if sl != nil && sl.Listing != nil && sl.Cid == hash {
			return sl.Listing.Item.Title
		}
	}
	return ""
}

func (v *UnifiedOrderView) fetchGuest(ctx context.Context, f contracts.OrderListFilter) ([]contracts.OrderSummary, int64, error) {
	if v.guestSvc == nil {
		return nil, 0, nil
	}
	if !v.guestSvc.IsEnabled(ctx) {
		return nil, 0, nil
	}

	filter := contracts.GuestOrderFilter{
		Page:     0,
		PageSize: 10000,
	}
	if f.State != "" {
		if st, ok := models.ParseGuestOrderState(f.State); ok {
			filter.State = &st
		}
	}

	orders, total, err := v.guestSvc.ListGuestOrders(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	sweepMap := v.batchSweepStatuses(orders)

	out := make([]contracts.OrderSummary, 0, len(orders))
	for _, g := range orders {
		out = append(out, convertGuestOrder(g, sweepMap[g.OrderToken]))
	}
	return out, total, nil
}

func (v *UnifiedOrderView) batchSweepStatuses(orders []models.GuestOrder) map[string]string {
	result := make(map[string]string, len(orders))
	if v.db == nil || len(orders) == 0 {
		return result
	}

	tokens := make([]string, 0, len(orders))
	for _, o := range orders {
		if o.SweepToAddress != "" {
			tokens = append(tokens, o.OrderToken)
		}
	}
	if len(tokens) == 0 {
		return result
	}

	var tasks []models.SweepTask
	_ = v.db.View(func(tx database.Tx) error {
		return tx.Read().Where("order_token IN ?", tokens).
			Select("order_token, status").Find(&tasks).Error
	})

	for _, t := range tasks {
		result[t.OrderToken] = string(t.Status)
	}
	return result
}

func convertGuestOrder(g models.GuestOrder, sweepStatus string) contracts.OrderSummary {
	items := make([]contracts.ItemBrief, 0, len(g.Items))
	for _, it := range g.Items {
		items = append(items, contracts.ItemBrief{
			Title:    it.ListingTitle,
			Quantity: it.Quantity,
		})
	}

	buyerName := "Guest"
	if g.ContactEmail != "" {
		buyerName = g.ContactEmail
	}

	if sweepStatus == "" && g.SweepToAddress != "" {
		sweepStatus = "pending"
	}

	updatedAt := g.CreatedAt
	if g.FundedAt != nil {
		updatedAt = *g.FundedAt
	}
	if g.CompletedAt != nil {
		updatedAt = *g.CompletedAt
	}

	return contracts.OrderSummary{
		ID:        "gst_" + g.OrderToken,
		Type:      "guest",
		State:     g.State.String(),
		BuyerName: buyerName,
		Items:     items,
		Total: contracts.PriceSummary{
			Amount:       g.PaymentAmount,
			CurrencyCode: g.PaymentCoin,
		},
		PaymentCoin:    g.PaymentCoin,
		TrackingNumber: g.TrackingNumber,
		SweepStatus:    sweepStatus,
		CreatedAt:      g.CreatedAt,
		UpdatedAt:      updatedAt,
	}
}

var _ contracts.UnifiedOrderViewService = (*UnifiedOrderView)(nil)
