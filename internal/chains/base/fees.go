package base

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/mobazha/mobazha3.0/libs/proxyclient"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// FeeProvider is an interface used for selecting fee (usually fee-per-byte)
// for a transaction.
type FeeProvider interface {
	// GetFee returns the appropriate fee for the given level.
	GetFee(level iwallet.FeeLevel) (iwallet.Amount, error)
}

// HardCodedFeeProvider is a basic implementation of the FeeProvider interface
// which returns hard coded fees from a map.
type HardCodedFeeProvider struct {
	feeMap map[iwallet.FeeLevel]iwallet.Amount
}

// NewHardCodedFeeProvider constructs a new HardCodedFeeProvider.
func NewHardCodedFeeProvider(Priority, Normal, Economic, SuperEconomic iwallet.Amount) FeeProvider {
	return &HardCodedFeeProvider{
		feeMap: map[iwallet.FeeLevel]iwallet.Amount{
			iwallet.FlPriority:      Priority,
			iwallet.FlNormal:        Normal,
			iwallet.FlEconomic:      Economic,
			iwallet.FLSuperEconomic: SuperEconomic,
		},
	}
}

// GetFee returns the appropriate fee for the given level.
func (fp *HardCodedFeeProvider) GetFee(level iwallet.FeeLevel) (iwallet.Amount, error) {
	fee, ok := fp.feeMap[level]
	if !ok {
		if int(level) < 0 {
			return iwallet.NewAmount(0), errors.New("negative fee")
		}
		return iwallet.NewAmount(int(level)), nil
	}
	return fee, nil
}

// APIFeeProvider is an implementation of the FeeProvider which returns fees from
// an API.
type APIFeeProvider struct {
	aPIEndpoint string
	cache       map[iwallet.FeeLevel]iwallet.Amount
	maxFee      iwallet.Amount
	lastQueried time.Time
	client      *http.Client
	mtx         sync.Mutex
}

// NewAPIFeeProvider returns a new APIFeeProvider.
func NewAPIFeeProvider(APIEndpoint string, maxFee iwallet.Amount) FeeProvider {
	return &APIFeeProvider{
		aPIEndpoint: APIEndpoint,
		maxFee:      maxFee,
		cache:       make(map[iwallet.FeeLevel]iwallet.Amount),
		mtx:         sync.Mutex{},
		client:      proxyclient.NewHttpClient(),
	}
}

// GetFee returns the appropriate fee for the given level.
func (fp *APIFeeProvider) GetFee(level iwallet.FeeLevel) (iwallet.Amount, error) {
	fp.mtx.Lock()
	defer fp.mtx.Unlock()

	fromCache := func() (iwallet.Amount, error) {
		fee, ok := fp.cache[level]
		if !ok {
			if int(level) < 0 {
				return iwallet.NewAmount(0), errors.New("negative fee")
			}
			return iwallet.NewAmount(int(level)), nil
		}
		if fee.Cmp(fp.maxFee) > 0 {
			return fp.maxFee, nil
		}
		return fee, nil
	}

	if fp.lastQueried.Add(time.Minute * 10).After(time.Now()) {
		return fromCache()
	}

	type apiResponse struct {
		Priority      int `json:"priority"`
		Normal        int `json:"normal"`
		Economic      int `json:"economic"`
		SuperEconomic int `json:"superEconomic"`
	}

	resp, err := fp.client.Get(fp.aPIEndpoint)
	if err != nil {
		return iwallet.NewAmount(0), err
	}
	var feeResponse apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&feeResponse); err != nil {
		return iwallet.NewAmount(0), err
	}

	fp.cache[iwallet.FlPriority] = iwallet.NewAmount(feeResponse.Priority)
	fp.cache[iwallet.FlNormal] = iwallet.NewAmount(feeResponse.Normal)
	fp.cache[iwallet.FlEconomic] = iwallet.NewAmount(feeResponse.Economic)
	fp.cache[iwallet.FLSuperEconomic] = iwallet.NewAmount(feeResponse.SuperEconomic)
	fp.lastQueried = time.Now()

	return fromCache()
}

// ExchangeRateFeeProvider is an implementation of the FeeProvider which targets a
// specific USD exchange rate for the fees.
type ExchangeRateFeeProvider struct {
	targetMap          map[iwallet.FeeLevel]float64
	maxFee             iwallet.Amount
	erp                ExchangeRateProvider
	coinType           iwallet.CoinType
	divisibility       float64
	avgTransactionSize float64
}

// NewExchangeRateFeeProvider returns a new ExchangeRateFeeProvider.
func NewExchangeRateFeeProvider(coinType iwallet.CoinType, divisibility int, erp ExchangeRateProvider, avgTransactionSize int,
	maxFeePerByte iwallet.Amount, PriorityUSDCents, NormalUSDCents, EconomicUSDCents, SuperEconomicUSDCents float64) FeeProvider {
	return &ExchangeRateFeeProvider{
		targetMap: map[iwallet.FeeLevel]float64{
			iwallet.FlPriority:      PriorityUSDCents,
			iwallet.FlNormal:        NormalUSDCents,
			iwallet.FlEconomic:      EconomicUSDCents,
			iwallet.FLSuperEconomic: SuperEconomicUSDCents,
		},
		maxFee:             maxFeePerByte,
		erp:                erp,
		coinType:           coinType,
		divisibility:       math.Pow10(divisibility),
		avgTransactionSize: float64(avgTransactionSize),
	}
}

// GetFee returns the appropriate fee for the given level.
func (fp *ExchangeRateFeeProvider) GetFee(level iwallet.FeeLevel) (iwallet.Amount, error) {
	target, ok := fp.targetMap[level]
	if !ok {
		if int(level) < 0 {
			return iwallet.NewAmount(0), errors.New("negative fee")
		}
		return iwallet.NewAmount(int(level)), nil
	}

	rateAmt, err := fp.erp.GetUSDRate(fp.coinType)
	if err != nil {
		return iwallet.NewAmount(0), err
	}

	rate := float64(rateAmt.Uint64())

	feePerByte := (((target / 100) / rate) * fp.divisibility) / fp.avgTransactionSize
	if feePerByte < 1 {
		return iwallet.NewAmount(1), nil
	}
	feeAmt := iwallet.NewAmount(uint64(feePerByte))
	if feeAmt.Cmp(fp.maxFee) > 0 {
		return fp.maxFee, nil
	}

	return feeAmt, nil
}
