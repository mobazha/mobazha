package coreiface

import (
	"context"

	"github.com/ipfs/kubo/core"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/multiwallet"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stripe/stripe-go/v82"
)

// CoreIface enumerates the interface of the MobazhaNode object in the Core package.
// It embeds contracts.NodeService (the aggregate interface suitable for both MobazhaNode
// and TenantService) and adds internal-only methods that depend on internal/ types.
//
// API handlers should prefer depending on contracts.NodeService where possible.
// CoreIface is kept for backward compatibility and for methods that require internal types.
type CoreIface interface {
	// Embed all public aggregate interfaces from pkg/contracts.
	// This ensures MobazhaNode implements contracts.NodeService automatically.
	contracts.NodeService

	// --- Internal-only methods (depend on internal/ types) ---

	Start()
	Stop(force bool) error
	IPFSNode() *core.IpfsNode
	DestroyNode()

	// DB returns the internal database handle.
	// TenantService would use shared DB with tenant scoping instead.
	DB() database.Database

	// Multiwallet returns the internal multiwallet instance.
	// In SaaS mode, this is a SharedMode multiwallet (keys only).
	Multiwallet() multiwallet.Multiwallet

	// ExchangeRates returns the internal exchange rate provider.
	ExchangeRates() *wallet.ExchangeRateProvider

	// UsingTorMode returns whether the node is using Tor.
	UsingTorMode() bool

	// Stripe methods (depend on stripe-go types)
	GetStripePublicKey() (string, error)
	GetStripeConnectURL() (string, error)
	CreateStripePaymentIntent(ctx context.Context, orderID models.OrderID, amount int64, currency string) (*stripe.PaymentIntent, error)
	HandleStripeWebhook(payload []byte, signature string) error
	UpdateOrderPaymentStatus(orderID models.OrderID, paymentIntentID string, status string) error
}

type NodeManagerIface interface {
	GetDefaultNode() CoreIface
	GetIPFSNode() *core.IpfsNode

	AddNode(nodeID string, node CoreIface)
	RemoveNode(nodeID string)
	GetNodes() map[string]CoreIface
	GetNode(nodeID string) (CoreIface, bool)

	// Config methods
	GetMaxImportZipSize() int64
	GetMaxImportVideoSize() int64
}
