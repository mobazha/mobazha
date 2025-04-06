package wallet

import (
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/mobazha/mobazha3.0/internal/models"
	iwallet "github.com/mobazha/mobazha3.0/internal/multiwallet/wallet-interface"

	mcfg "github.com/mobazha/mobazha3.0/internal/config"
)

var (
	expectedBTCRates = map[models.CurrencyCode]iwallet.Amount{
		models.CurrencyCode("BTC"): iwallet.NewAmount("100000000"),
		models.CurrencyCode("BCH"): iwallet.NewAmount("3158891500"),
		models.CurrencyCode("LTC"): iwallet.NewAmount("11216324600"),
		models.CurrencyCode("ETH"): iwallet.NewAmount("42353160000000002584"),
		models.CurrencyCode("ZEC"): iwallet.NewAmount("12877108999"),
		models.CurrencyCode("USD"): iwallet.NewAmount("1286307"),
		models.CurrencyCode("EUR"): iwallet.NewAmount("1144457"),
		models.CurrencyCode("JPY"): iwallet.NewAmount("139831116"),
		models.CurrencyCode("CNY"): iwallet.NewAmount("8843982"),
	}

	expectedBCHRates = map[models.CurrencyCode]iwallet.Amount{
		models.CurrencyCode("BTC"): iwallet.NewAmount("3165667"),
		models.CurrencyCode("BCH"): iwallet.NewAmount("100000000"),
		models.CurrencyCode("LTC"): iwallet.NewAmount("355071536"),
		models.CurrencyCode("ETH"): iwallet.NewAmount("1340760200215803631"),
		models.CurrencyCode("ZEC"): iwallet.NewAmount("407646448"),
		models.CurrencyCode("USD"): iwallet.NewAmount("40720"),
		models.CurrencyCode("EUR"): iwallet.NewAmount("36229"),
		models.CurrencyCode("JPY"): iwallet.NewAmount("4426588"),
		models.CurrencyCode("CNY"): iwallet.NewAmount("279971"),
	}
)

var tickerEndpoint = mcfg.DefaultNetConfig().GetExchangeRateProviders()[0]

func TestExchangeRateProvider_GetRate(t *testing.T) {
	mockedHTTPClient := http.Client{}
	httpmock.ActivateNonDefault(&mockedHTTPClient)

	defer httpmock.DeactivateAndReset()

	resp := MockExchangeRateResponse
	httpmock.RegisterResponder(http.MethodGet, tickerEndpoint,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(http.StatusInternalServerError, resp)
		},
	)

	provider := NewExchangeRateProvider([]string{tickerEndpoint})
	obAPI, ok := provider.providers[0].(*openBazaarAPI)
	if !ok {
		t.Fatal("Type assertion failure provider 0 is not openBazaarAPI")
	}
	obAPI.client = &mockedHTTPClient

	rate, err := provider.GetRate("BTC", "USD", true)
	if err != nil {
		t.Fatal(err)
	}

	expectedBTCUSD := iwallet.NewAmount(1286307)
	if rate.Cmp(expectedBTCUSD) != 0 {
		t.Errorf("Returned incorrect rate. Expected %s, got %s", expectedBTCUSD, rate)
	}

	rate, err = provider.GetRate("BCH", "USD", true)
	if err != nil {
		t.Fatal(err)
	}

	expectedBCHUSD := iwallet.NewAmount(40720)
	if rate.Cmp(expectedBCHUSD) != 0 {
		t.Errorf("Returned incorrect rate. Expected %s, got %s", expectedBCHUSD, rate)
	}
}

func TestExchangeRateProvider_GetAllRates(t *testing.T) {
	mockedHTTPClient := http.Client{}
	httpmock.ActivateNonDefault(&mockedHTTPClient)

	defer httpmock.DeactivateAndReset()

	resp := MockExchangeRateResponse
	httpmock.RegisterResponder(http.MethodGet, tickerEndpoint,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(http.StatusInternalServerError, resp)
		},
	)

	provider := NewExchangeRateProvider([]string{tickerEndpoint})
	obAPI, ok := provider.providers[0].(*openBazaarAPI)
	if !ok {
		t.Fatal("Type assertion failure provider 0 is not openBazaarAPI")
	}
	obAPI.client = &mockedHTTPClient

	btcRates, err := provider.GetAllRates("BTC", true)
	if err != nil {
		t.Fatal(err)
	}

	for cc, expected := range expectedBTCRates {
		rate, ok := btcRates[cc]
		if !ok {
			t.Fatalf("Currency %s not in returned map", cc)
		}

		if rate.Cmp(expected) != 0 {
			t.Errorf("Rate %s incorrected. Expected %s, got %s", cc, expected, rate)
		}
	}

	bchRates, err := provider.GetAllRates("BCH", true)
	if err != nil {
		t.Fatal(err)
	}

	for cc, expected := range expectedBCHRates {
		rate, ok := bchRates[cc]
		if !ok {
			t.Fatalf("Currency %s not in returned map", cc)
		}

		if rate.Cmp(expected) != 0 {
			t.Errorf("Rate %s incorrected. Expected %s, got %s", cc, expected, rate)
		}
	}
}

func TestExchangeRateProvider_openBazaarAPI(t *testing.T) {
	mockedHTTPClient := http.Client{}
	httpmock.ActivateNonDefault(&mockedHTTPClient)

	defer httpmock.DeactivateAndReset()

	api := openBazaarAPI{
		url:    tickerEndpoint,
		client: &mockedHTTPClient,
	}

	resp := MockExchangeRateResponse
	httpmock.RegisterResponder(http.MethodGet, tickerEndpoint,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(http.StatusInternalServerError, resp)
		},
	)

	btcRates, err := api.fetchRates("BTC")
	if err != nil {
		t.Fatal(err)
	}

	for cc, expected := range expectedBTCRates {
		rate, ok := btcRates[cc]
		if !ok {
			t.Fatalf("Currency %s not in returned map", cc)
		}

		if rate.Cmp(expected) != 0 {
			t.Errorf("Rate %s incorrected. Expected %s, got %s", cc, expected, rate)
		}
	}

	bchRates, err := api.fetchRates("BCH")
	if err != nil {
		t.Fatal(err)
	}

	for cc, expected := range expectedBCHRates {
		rate, ok := bchRates[cc]
		if !ok {
			t.Fatalf("Currency %s not in returned map", cc)
		}

		if rate.Cmp(expected) != 0 {
			t.Errorf("Rate %s incorrected. Expected %s, got %s", cc, expected, rate)
		}
	}
}
