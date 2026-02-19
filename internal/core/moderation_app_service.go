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
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// DHT callback types abstract IPFS-specific moderator discovery operations.
type (
	DHTAnnounceModeratorFunc    func(ctx context.Context) error
	DHTRemoveModeratorFunc      func(ctx context.Context) error
	DHTFindModeratorsAsyncFunc  func(ctx context.Context, maxCount int) <-chan peer.ID
	UpdateAllListingsFunc       func(updateFunc func(l *pb.Listing) (bool, error), done chan<- struct{}) error
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
	announceAsModerator   DHTAnnounceModeratorFunc
	removeAsModerator     DHTRemoveModeratorFunc
	findModeratorsAsync   DHTFindModeratorsAsyncFunc
	updateAllListings     UpdateAllListingsFunc
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
	AnnounceAsModerator   DHTAnnounceModeratorFunc
	RemoveAsModerator     DHTRemoveModeratorFunc
	FindModeratorsAsync   DHTFindModeratorsAsyncFunc
	UpdateAllListings     UpdateAllListingsFunc
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
		announceAsModerator:   cfg.AnnounceAsModerator,
		removeAsModerator:     cfg.RemoveAsModerator,
		findModeratorsAsync:   cfg.FindModeratorsAsync,
		updateAllListings:     cfg.UpdateAllListings,
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

		if err := tx.SetProfile(profile); err != nil {
			return err
		}

		if s.announceAsModerator != nil {
			return s.announceAsModerator(ctx)
		}
		return nil
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

		if err := tx.SetProfile(profile); err != nil {
			return err
		}

		if s.removeAsModerator != nil {
			return s.removeAsModerator(ctx)
		}
		return nil
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

func (s *ModerationAppService) GetModerators(ctx context.Context) []peer.ID {
	var mods []peer.ID
	for mod := range s.GetModeratorsAsync(ctx) {
		mods = append(mods, mod)
	}
	return mods
}

func (s *ModerationAppService) GetModeratorsAsync(ctx context.Context) <-chan peer.ID {
	if s.findModeratorsAsync != nil {
		return s.findModeratorsAsync(ctx, maxModerators)
	}
	ch := make(chan peer.ID)
	close(ch)
	return ch
}

func (s *ModerationAppService) SetModeratorsOnListings(mods []peer.ID, done chan struct{}) error {
	modStrs := make([]string, 0, len(mods))
	for _, mod := range mods {
		modStrs = append(modStrs, mod.String())
	}

	return s.updateAllListings(func(listing *pb.Listing) (bool, error) {
		listing.Moderators = modStrs
		return true, nil
	}, done)
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
