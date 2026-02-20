package core

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testVendorPeerID = "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG"

func newTestShoppingCartAppService(t *testing.T) *ShoppingCartAppService {
	t.Helper()
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewShoppingCartAppService(ShoppingCartAppServiceConfig{
		DB:       db,
		EventBus: events.NewBus(),
		NodeID:   "test-cart-svc",
	})
}

func mustPeerID(t *testing.T, s string) peer.ID {
	t.Helper()
	pid, err := peer.Decode(s)
	require.NoError(t, err)
	return pid
}

// ── GetCarts ────────────────────────────────────────────────────

func TestShoppingCartAppService_GetCarts_EmptyDB(t *testing.T) {
	svc := newTestShoppingCartAppService(t)
	carts, err := svc.GetCarts()
	require.NoError(t, err)
	assert.Empty(t, carts)
}

// ── AddToCart / GetCarts ────────────────────────────────────────

func TestShoppingCartAppService_AddToCart_NewItem(t *testing.T) {
	svc := newTestShoppingCartAppService(t)
	vendorID := mustPeerID(t, testVendorPeerID)
	item := models.ShoppingCartItem{Slug: "tshirt-blue", Quantity: "2"}

	require.NoError(t, svc.AddToCart(vendorID, item))

	carts, err := svc.GetCarts()
	require.NoError(t, err)
	require.Len(t, carts, 1)
	assert.Equal(t, vendorID.String(), carts[0].VendorID)
	require.Len(t, carts[0].Items, 1)
	assert.Equal(t, "tshirt-blue", carts[0].Items[0].Slug)
	assert.Equal(t, "2", carts[0].Items[0].Quantity)
}

func TestShoppingCartAppService_AddToCart_UpdateExistingItem(t *testing.T) {
	svc := newTestShoppingCartAppService(t)
	vendorID := mustPeerID(t, testVendorPeerID)

	require.NoError(t, svc.AddToCart(vendorID, models.ShoppingCartItem{Slug: "tshirt-blue", Quantity: "1"}))
	require.NoError(t, svc.AddToCart(vendorID, models.ShoppingCartItem{Slug: "tshirt-blue", Quantity: "5"}))

	carts, err := svc.GetCarts()
	require.NoError(t, err)
	require.Len(t, carts, 1)
	require.Len(t, carts[0].Items, 1)
	assert.Equal(t, "5", carts[0].Items[0].Quantity)
}

func TestShoppingCartAppService_AddToCart_MultipleItems(t *testing.T) {
	svc := newTestShoppingCartAppService(t)
	vendorID := mustPeerID(t, testVendorPeerID)

	require.NoError(t, svc.AddToCart(vendorID, models.ShoppingCartItem{Slug: "tshirt-blue", Quantity: "1"}))
	require.NoError(t, svc.AddToCart(vendorID, models.ShoppingCartItem{Slug: "hat-red", Quantity: "3"}))

	carts, err := svc.GetCarts()
	require.NoError(t, err)
	require.Len(t, carts, 1)
	assert.Len(t, carts[0].Items, 2)
}

// ── GetCartsTotalItemsCount ─────────────────────────────────────

func TestShoppingCartAppService_GetCartsTotalItemsCount(t *testing.T) {
	svc := newTestShoppingCartAppService(t)
	vendorID := mustPeerID(t, testVendorPeerID)

	require.NoError(t, svc.AddToCart(vendorID, models.ShoppingCartItem{Slug: "a", Quantity: "1"}))
	require.NoError(t, svc.AddToCart(vendorID, models.ShoppingCartItem{Slug: "b", Quantity: "1"}))

	total, err := svc.GetCartsTotalItemsCount()
	require.NoError(t, err)
	assert.Equal(t, 2, total)
}

func TestShoppingCartAppService_GetCartsTotalItemsCount_Empty(t *testing.T) {
	svc := newTestShoppingCartAppService(t)
	total, err := svc.GetCartsTotalItemsCount()
	require.NoError(t, err)
	assert.Equal(t, 0, total)
}

// ── RemoveCartItem ──────────────────────────────────────────────

func TestShoppingCartAppService_RemoveCartItem(t *testing.T) {
	svc := newTestShoppingCartAppService(t)
	vendorID := mustPeerID(t, testVendorPeerID)
	item1 := models.ShoppingCartItem{Slug: "a", Quantity: "1"}
	item2 := models.ShoppingCartItem{Slug: "b", Quantity: "2"}

	require.NoError(t, svc.AddToCart(vendorID, item1))
	require.NoError(t, svc.AddToCart(vendorID, item2))
	require.NoError(t, svc.RemoveCartItem(vendorID, item1))

	carts, err := svc.GetCarts()
	require.NoError(t, err)
	require.Len(t, carts, 1)
	assert.Len(t, carts[0].Items, 1)
	assert.Equal(t, "b", carts[0].Items[0].Slug)
}

func TestShoppingCartAppService_RemoveCartItem_LastItem_DeletesRecord(t *testing.T) {
	svc := newTestShoppingCartAppService(t)
	vendorID := mustPeerID(t, testVendorPeerID)
	item := models.ShoppingCartItem{Slug: "only-item", Quantity: "1"}

	require.NoError(t, svc.AddToCart(vendorID, item))
	require.NoError(t, svc.RemoveCartItem(vendorID, item))

	carts, err := svc.GetCarts()
	require.NoError(t, err)
	assert.Empty(t, carts)
}

func TestShoppingCartAppService_RemoveCartItem_NotFound(t *testing.T) {
	svc := newTestShoppingCartAppService(t)
	vendorID := mustPeerID(t, testVendorPeerID)

	require.NoError(t, svc.AddToCart(vendorID, models.ShoppingCartItem{Slug: "a", Quantity: "1"}))
	require.NoError(t, svc.RemoveCartItem(vendorID, models.ShoppingCartItem{Slug: "nonexistent", Quantity: "1"}))

	carts, err := svc.GetCarts()
	require.NoError(t, err)
	require.Len(t, carts, 1)
	assert.Len(t, carts[0].Items, 1)
}

// ── ClearCarts ──────────────────────────────────────────────────

func TestShoppingCartAppService_ClearCarts(t *testing.T) {
	svc := newTestShoppingCartAppService(t)
	vendorID := mustPeerID(t, testVendorPeerID)

	require.NoError(t, svc.AddToCart(vendorID, models.ShoppingCartItem{Slug: "a", Quantity: "1"}))
	require.NoError(t, svc.AddToCart(vendorID, models.ShoppingCartItem{Slug: "b", Quantity: "2"}))
	require.NoError(t, svc.ClearCarts(vendorID))

	carts, err := svc.GetCarts()
	require.NoError(t, err)
	assert.Empty(t, carts)
}

// ── ClearAllCarts ───────────────────────────────────────────────

func TestShoppingCartAppService_ClearAllCarts(t *testing.T) {
	svc := newTestShoppingCartAppService(t)
	vendorID := mustPeerID(t, testVendorPeerID)

	require.NoError(t, svc.AddToCart(vendorID, models.ShoppingCartItem{Slug: "a", Quantity: "1"}))
	require.NoError(t, svc.ClearAllCarts())

	carts, err := svc.GetCarts()
	require.NoError(t, err)
	assert.Empty(t, carts)
}
