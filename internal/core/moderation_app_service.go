package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/libs/proxyclient"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ModerationAppService encapsulates moderator management logic.
type ModerationAppService struct {
	db                    database.Database
	publish               PublishFunc
	nodeID                string
	verifiedModEndpoint   string
	exchangeRates         *wallet.ExchangeRateProvider

	getMyProfile          GetMyProfileFunc
	getAcceptedCurrencies GetAcceptedCurrenciesFunc
}

// ModerationAppServiceConfig holds dependencies for constructing a ModerationAppService.
type ModerationAppServiceConfig struct {
	DB                    database.Database
	Publish               PublishFunc
	NodeID                string
	VerifiedModEndpoint   string
	ExchangeRates         *wallet.ExchangeRateProvider

	GetMyProfile          GetMyProfileFunc
	GetAcceptedCurrencies GetAcceptedCurrenciesFunc
}

func NewModerationAppService(cfg ModerationAppServiceConfig) *ModerationAppService {
	return &ModerationAppService{
		db:                    cfg.DB,
		publish:               cfg.Publish,
		nodeID:                cfg.NodeID,
		verifiedModEndpoint:   cfg.VerifiedModEndpoint,
		exchangeRates:         cfg.ExchangeRates,
		getMyProfile:          cfg.GetMyProfile,
		getAcceptedCurrencies: cfg.GetAcceptedCurrencies,
	}
}

func (s *ModerationAppService) SetSelfAsModerator(ctx context.Context, modInfo *models.ModeratorInfo, done chan struct{}) error {
	if (int(modInfo.Fee.FeeType) == 0 || int(modInfo.Fee.FeeType) == 2) && modInfo.Fee.FixedFee == nil {
		maybeCloseDone(done)
		return errors.New("fixed fee must be set when using a fixed fee type")
	}

	enabledCoins, _ := s.getAcceptedCurrencies()
	err := s.db.Update(func(tx database.Tx) error {
		profile, err := tx.GetProfile()
		if err != nil {
			return err
		}

		validCurrencies := map[string]bool{}
		for _, currency := range profile.Currencies {
			validCurrencies[currency] = true
		}

		var currencies []string
		currencies = profile.Currencies
		for _, cc := range modInfo.AcceptedCurrencies {
			if _, ok := validCurrencies[normalizeCurrencyCode(cc)]; ok {
				currencies = append(currencies, normalizeCurrencyCode(cc))
			}
		}

		if len(currencies) == 0 {
			currencies = append(currencies, enabledCoins...)
		}
		modInfo.AcceptedCurrencies = currencies

		profile.ModeratorInfo = modInfo
		profile.Moderator = true

		return tx.SetProfile(profile)
	})
	if err != nil {
		maybeCloseDone(done)
		return err
	}
	s.publish(done)
	return nil
}

func (s *ModerationAppService) RemoveSelfAsModerator(ctx context.Context, done chan<- struct{}) error {
	err := s.db.Update(func(tx database.Tx) error {
		profile, err := tx.GetProfile()
		if err != nil {
			return err
		}
		profile.Moderator = false

		return tx.SetProfile(profile)
	})
	if err != nil {
		maybeCloseDone(done)
		return err
	}
	s.publish(done)
	return nil
}

func (s *ModerationAppService) IsModerator() bool {
	myProfile, err := s.getMyProfile()
	if err != nil {
		return false
	}
	return myProfile.Moderator
}

func (s *ModerationAppService) GetVerifiedModerators(ctx context.Context) []peer.ID {
	if len(s.verifiedModEndpoint) == 0 {
		return []peer.ID{}
	}

	client := proxyclient.NewHttpClient()
	client.Timeout = time.Second * 30
	resp, err := client.Get(s.verifiedModEndpoint)
	if err != nil {
		return []peer.ID{}
	}
	defer resp.Body.Close()

	type ModResp struct {
		Success bool     `json:"success"`
		Total   int32    `json:"total"`
		Results []string `json:"results"`
	}
	var modResp ModResp

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&modResp); err != nil {
		return []peer.ID{}
	}

	var mods []peer.ID
	for _, id := range modResp.Results {
		mod, err := peer.Decode(id)
		if err != nil {
			continue
		}
		mods = append(mods, mod)
	}
	return mods
}

func (s *ModerationAppService) GetModerators(_ context.Context) []peer.ID {
	return []peer.ID{}
}

func (s *ModerationAppService) GetModeratorsAsync(_ context.Context) <-chan peer.ID {
	ch := make(chan peer.ID)
	close(ch)
	return ch
}

func (s *ModerationAppService) GetModeratorFee(total iwallet.Amount, currencyCode string) (iwallet.Amount, error) {
	myProfile, err := s.getMyProfile()
	if err != nil {
		return iwallet.NewAmount(0), fmt.Errorf("failed to get my profile, %s", err)
	}
	moderatorInfo := myProfile.ModeratorInfo

	switch moderatorInfo.Fee.FeeType {
	case models.PercentageFee:
		feePercent := new(big.Float).Mul(big.NewFloat(moderatorInfo.Fee.Percentage), big.NewFloat(0.01))
		var feePercentAmt, _ = new(big.Float).Mul(new(big.Float).SetInt64(total.Int64()), feePercent).Int(nil)
		return iwallet.NewAmount(feePercentAmt), nil

	case models.FixedFee:
		return s.calculateFixedFee(total, currencyCode, moderatorInfo)

	case models.FixedPlusPercentageFee:
		fixedPart, err := s.calculateFixedFee(total, currencyCode, moderatorInfo)
		if err != nil {
			return iwallet.NewAmount(0), err
		}
		feePercent := new(big.Float).Mul(big.NewFloat(moderatorInfo.Fee.Percentage), big.NewFloat(0.01))
		var feePercentAmt, _ = new(big.Float).Mul(new(big.Float).SetInt64(total.Int64()), feePercent).Int(nil)
		feeTotal := fixedPart.Add(iwallet.NewAmount(feePercentAmt))
		if feeTotal.Cmp(total) > 0 {
			return iwallet.NewAmount(0), errors.New("fixed moderator fee exceeds transaction amount")
		}
		return feeTotal, nil

	default:
		return iwallet.NewAmount(0), errors.New("unrecognized fee type")
	}
}

func (s *ModerationAppService) calculateFixedFee(total iwallet.Amount, currencyCode string, modInfo *models.ModeratorInfo) (iwallet.Amount, error) {
	paymentCurrency, err := models.CurrencyDefinitions.Lookup(currencyCode)
	if err != nil {
		return iwallet.NewAmount(0), err
	}

	convertedModFee, err := wallet.ConvertCurrencyAmount(modInfo.Fee.FixedFee, paymentCurrency, s.exchangeRates)
	if err != nil {
		return iwallet.NewAmount(0), fmt.Errorf("convert moderator fee into transaction currency (%s): %s", paymentCurrency.String(), err)
	}
	if convertedModFee.Cmp(total) > 0 {
		return iwallet.NewAmount(0), errors.New("fixed moderator fee exceeds transaction amount")
	}
	return convertedModFee, nil
}

