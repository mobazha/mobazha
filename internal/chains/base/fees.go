package base

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/mobazha/mobazha/libs/proxyclient"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
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
