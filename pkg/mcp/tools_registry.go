package mcp

func getAllToolRegistrars(bf BridgeFactory, opts *ServerOptions) []ToolRegistrar {
	if opts != nil && opts.ToolProfile == ToolProfilePrivateDistribution {
		return getPrivateDistributionToolRegistrars(bf)
	}

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
	all = append(all, fulfillmentToolRegistrars(bf)...)

	if opts != nil && opts.SearchURL != "" {
		searchBridge := NewHTTPBridge(opts.SearchURL, "", "", nil)
		all = append(all, searchToolRegistrars(searchBridge)...)
	}

	if opts != nil && opts.Shopping != nil && opts.Shopping.DemoStorePeerID != "" {
		var searchBridge Bridge
		if opts.SearchURL != "" {
			searchBridge = NewHTTPBridge(opts.SearchURL, "", "", nil)
		}
		storeURL := opts.StoreGatewayURL
		if storeURL == "" {
			storeURL = "http://localhost:4002"
		}
		// storeBridge targets the demo store; peerID is set so SaaS gateway
		// routes to the correct tenant node via X-Store-PeerID.
		storeBridge := NewHTTPBridge(storeURL, "", opts.Shopping.DemoStorePeerID, nil)
		signer := NewQuoteTokenSigner(opts.QuoteTokenSecret)
		all = append(all, shoppingToolRegistrars(searchBridge, storeBridge, *opts.Shopping, signer)...)
	}

	return all
}

func getPrivateDistributionToolRegistrars(bf BridgeFactory) []ToolRegistrar {
	var all []ToolRegistrar
	all = append(all, profileToolRegistrars(bf)...)
	all = append(all, listingsToolRegistrars(bf)...)
	all = append(all, discountsToolRegistrars(bf)...)
	all = append(all, collectionsToolRegistrars(bf)...)
	all = append(all, settingsToolRegistrars(bf)...)
	return all
}
