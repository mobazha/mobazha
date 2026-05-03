//go:build !private_distribution

package mcp

func getAllToolRegistrars(bf BridgeFactory, opts *ServerOptions) []ToolRegistrar {
	var all []ToolRegistrar
	all = append(all, listingsToolRegistrars(bf)...)
	all = append(all, ordersToolRegistrars(bf)...)
	all = append(all, walletToolRegistrars(bf)...)
	all = append(all, profileToolRegistrars(bf)...)
	all = append(all, chatToolRegistrars(bf)...)
	all = append(all, notificationsToolRegistrars(bf)...)
	all = append(all, exchangeToolRegistrars(bf)...)
	all = append(all, discountsToolRegistrars(bf)...)
	all = append(all, collectionsToolRegistrars(bf)...)
	all = append(all, settingsToolRegistrars(bf)...)
	all = append(all, fiatToolRegistrars(bf)...)

	if opts != nil && opts.SearchURL != "" {
		searchBridge := NewHTTPBridge(opts.SearchURL, "", "", nil)
		all = append(all, searchToolRegistrars(searchBridge)...)
	}

	return all
}
