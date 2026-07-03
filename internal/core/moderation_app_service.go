package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/wallet"
	"github.com/mobazha/mobazha/libs/proxyclient"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ModerationAppService encapsulates moderator management logic.
type ModerationAppService struct {
	db                  database.Database
	publish             PublishFunc
	nodeID              string
	verifiedModEndpoint string
	exchangeRates       *wallet.ExchangeRateProvider
}

// ModerationAppServiceConfig holds dependencies for constructing a ModerationAppService.
type ModerationAppServiceConfig struct {
	DB                  database.Database
	Publish             PublishFunc
	NodeID              string
	VerifiedModEndpoint string
	ExchangeRates       *wallet.ExchangeRateProvider
}

func NewModerationAppService(cfg ModerationAppServiceConfig) *ModerationAppService {
	return &ModerationAppService{
		db:                  cfg.DB,
		publish:             cfg.Publish,
		nodeID:              cfg.NodeID,
		verifiedModEndpoint: cfg.VerifiedModEndpoint,
		exchangeRates:       cfg.ExchangeRates,
	}
}

func validateModeratorInfo(modInfo *models.ModeratorInfo) error {
	if modInfo == nil {
		return errors.New("moderator info is required")
	}
	if (modInfo.Fee.FeeType == models.FixedFee || modInfo.Fee.FeeType == models.FixedPlusPercentageFee) && modInfo.Fee.FixedFee == nil {
		return errors.New("fixed fee must be set when using a fixed fee type")
	}
	if len(modInfo.Description) > AboutMaxCharacters {
		return fmt.Errorf("moderatorinfo description exceeds %d characters", AboutMaxCharacters)
	}
	if len(modInfo.TermsAndConditions) > PolicyMaxCharacters {
		return fmt.Errorf("moderatorinfo termsandconditions exceeds %d characters", PolicyMaxCharacters)
	}
	if len(modInfo.Languages) > MaxListItems {
		return fmt.Errorf("moderatorinfo languages exceeds %d items", MaxListItems)
	}
	for _, l := range modInfo.Languages {
		if len(l) > WordMaxCharacters {
			return fmt.Errorf("moderatorinfo language exceeds %d characters", WordMaxCharacters)
		}
	}
	if len(modInfo.AcceptedCurrencies) > MaxListItems {
		return fmt.Errorf("moderatorinfo acceptedCurrencies exceeds %d items", MaxListItems)
	}
	for _, c := range modInfo.AcceptedCurrencies {
		if len(c) > WordMaxCharacters {
			return fmt.Errorf("moderatorinfo acceptedCurrency exceeds %d characters", WordMaxCharacters)
		}
	}
	if modInfo.Fee.FixedFee != nil {
		if len(modInfo.Fee.FixedFee.Currency.Name) > WordMaxCharacters {
			return fmt.Errorf("moderatorinfo fee currency name exceeds %d characters", WordMaxCharacters)
		}
		if len(string(modInfo.Fee.FixedFee.Currency.CurrencyType)) > WordMaxCharacters {
			return fmt.Errorf("moderatorinfo fee currency type exceeds %d characters", WordMaxCharacters)
		}
		if len(modInfo.Fee.FixedFee.Currency.Code.String()) > WordMaxCharacters {
			return fmt.Errorf("moderatorinfo fee currency code exceeds %d characters", WordMaxCharacters)
		}
	}
	return nil
}

func (s *ModerationAppService) SetSelfAsModerator(ctx context.Context, modInfo *models.ModeratorInfo, done chan struct{}) error {
	if err := validateModeratorInfo(modInfo); err != nil {
		maybeCloseDone(done)
		return err
	}

	enabledCoins, _ := s.queryAcceptedCurrencies()

	validCurrencies := map[string]bool{}
	for _, c := range enabledCoins {
		validCurrencies[normalizeCurrencyCode(c)] = true
	}

	var accepted []string
	for _, cc := range modInfo.AcceptedCurrencies {
		if validCurrencies[normalizeCurrencyCode(cc)] {
			accepted = append(accepted, normalizeCurrencyCode(cc))
		}
	}
	if len(accepted) == 0 {
		accepted = enabledCoins
	}
	modInfo.AcceptedCurrencies = accepted

	err := s.db.Update(func(tx database.Tx) error {
		profile, err := tx.GetProfile()
		if err != nil {
			return err
		}

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
	profile, err := s.getProfile()
	if err != nil {
		return false
	}
	return profile.Moderator
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

	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		return []peer.ID{}
	}

	var mods []peer.ID
	for _, id := range parseVerifiedModeratorPeerIDs(body) {
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
	profile, err := s.getProfile()
	if err != nil {
		return iwallet.NewAmount(0), fmt.Errorf("failed to get my profile, %s", err)
	}
	moderatorInfo := profile.ModeratorInfo

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

func (s *ModerationAppService) getProfile() (*models.Profile, error) {
	var profile *models.Profile
	err := s.db.View(func(tx database.Tx) error {
		var err error
		profile, err = tx.GetProfile()
		return err
	})
	return profile, err
}

func (s *ModerationAppService) queryAcceptedCurrencies() ([]string, error) {
	var records []models.ReceivingAccount
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	var currencies []string
	for _, record := range records {
		currencies = append(currencies, record.AcceptedCurrencies()...)
	}
	return currencies, nil
}
