package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/op/go-logging"
	"google.golang.org/protobuf/proto"
)

var log = logging.MustGetLogger("UTILS")

// ValidateRating returns an error if the rating is invalid, otherwise nil.
func ValidateRating(rating *pb.Rating) error {
	if rating.VendorID == nil || rating.VendorID.Pubkeys == nil {
		return errors.New("invalid vendor ID")
	}

	if rating.VendorSig == nil || rating.VendorSig.RatingKey == nil {
		return errors.New("invalid vendor signature")
	}

	if rating.Overall < 1 || rating.Overall > 5 {
		return errors.New("overall rating out of range")
	}
	if rating.Quality < 1 || rating.Quality > 5 {
		return errors.New("quality rating out of range")
	}
	if rating.Description < 1 || rating.Description > 5 {
		return errors.New("description rating out of range")
	}
	if rating.DeliverySpeed < 1 || rating.DeliverySpeed > 5 {
		return errors.New("delivery speed rating out of range")
	}
	if rating.CustomerService < 1 || rating.CustomerService > 5 {
		return errors.New("customer service rating out of range")
	}
	if len(rating.Review) > 10000 {
		return errors.New("review greater than max characters")
	}

	// Validate the vendor's signature
	vendorKey, err := crypto.UnmarshalPublicKey(rating.VendorID.Pubkeys.Identity)
	if err != nil {
		return err
	}

	cpy := proto.Clone(rating.VendorSig)
	cpy.(*pb.RatingSignature).VendorSignature = nil
	ser, err := proto.Marshal(cpy)
	if err != nil {
		return err
	}
	valid, err := vendorKey.Verify(ser, rating.VendorSig.VendorSignature)
	if !valid || err != nil {
		return errors.New("invalid vendor signature")
	}

	// Validate vendor peerID matches pubkey
	id, err := peer.Decode(rating.VendorID.PeerID)
	if err != nil {
		return err
	}
	if !id.MatchesPublicKey(vendorKey) {
		return errors.New("vendor ID does not match public key")
	}

	// Validate buyer signature if not anonymous
	if rating.BuyerID != nil {
		if rating.BuyerID.Pubkeys == nil {
			return errors.New("buyer public key is nil")
		}
		buyerKey, err := crypto.UnmarshalPublicKey(rating.BuyerID.Pubkeys.Identity)
		if err != nil {
			return err
		}
		ratingSigHash := sha256.Sum256(rating.VendorSig.RatingKey)
		valid, err = buyerKey.Verify(ratingSigHash[:], rating.BuyerSig)
		if !valid || err != nil {
			return errors.New("invalid buyer signature")
		}

		// Validate buyer peerID matches pubkey
		id, err := peer.Decode(rating.BuyerID.PeerID)
		if err != nil {
			return err
		}
		if !id.MatchesPublicKey(buyerKey) {
			return errors.New("buyer ID does not match public key")
		}
	}

	// Validate rating signature
	cpy = proto.Clone(rating)
	cpy.(*pb.Rating).RatingSignature = nil
	ser, err = proto.Marshal(cpy)
	if err != nil {
		return err
	}
	ratingKey, err := btcec.ParsePubKey(rating.VendorSig.RatingKey)
	if err != nil {
		return err
	}
	sig, err := ecdsa.ParseSignature(rating.RatingSignature)
	if err != nil {
		return err
	}
	hashed := sha256.Sum256(ser)
	valid = sig.Verify(hashed[:], ratingKey)
	if !valid {
		return errors.New("invalid rating signature")
	}

	return nil
}

// ValidateBuyerID validates the ID is well formed and contains a valid signature.
func ValidateBuyerID(id *pb.ID) error {
	if id.Pubkeys == nil {
		return errors.New("buyer id pubkeys is nil")
	}
	idPubkey, err := crypto.UnmarshalPublicKey(id.Pubkeys.Identity)
	if err != nil {
		return fmt.Errorf("invalid buyer ID pubkey: %s", err)
	}
	pid, err := peer.IDFromPublicKey(idPubkey)
	if err != nil {
		return fmt.Errorf("invalid buyer ID pubkey: %s", err)
	}
	if pid.String() != id.PeerID {
		return errors.New("buyer ID does not match pubkey")
	}
	escrowPubkey, err := btcec.ParsePubKey(id.Pubkeys.Escrow)
	if err != nil {
		return errors.New("invalid buyer escrow pubkey")
	}
	sig, err := ecdsa.ParseSignature(id.Sig)
	if err != nil {
		return errors.New("invalid buyer ID signature")
	}
	idHash := sha256.Sum256([]byte(id.PeerID))
	valid := sig.Verify(idHash[:], escrowPubkey)
	if !valid {
		return errors.New("invalid buyer ID signature")
	}
	return nil
}

func ValidatePayment(order *pb.OrderOpen, paymentSent *pb.PaymentSent, escrowTimeoutHours uint32, wal iwallet.Wallet) error {
	if paymentSent.Amount == "" {
		return errors.New("payment amount not set")
	}
	if ok := validateBigString(paymentSent.Amount); !ok {
		return errors.New("payment amount not valid")
	}
	if paymentSent.ToAddress == "" {
		return errors.New("order payment address is empty")
	}
	chaincode, err := hex.DecodeString(paymentSent.Chaincode)
	if err != nil {
		return fmt.Errorf("chaincode parse error: %s", err)
	}

	if wal.CoinCategory() == iwallet.CoinCategorySolana || wal.CoinCategory() == iwallet.CoinCategoryEthereum {
		return validateEscrowPayment(order, paymentSent, wal)
	} else if wal.CoinCategory() == iwallet.CoinCategoryStripe {
		return validateFiatPayment(order, paymentSent)
	} else {
		return validateBTCLikePayment(order, paymentSent, chaincode, wal, escrowTimeoutHours)
	}
}

// validateBTCLikePayment 验证BTC类支付
func validateBTCLikePayment(order *pb.OrderOpen, paymentSent *pb.PaymentSent, chaincode []byte, wal iwallet.Wallet, escrowTimeoutHours uint32) error {
	vendorEscrowPubkey, err := btcec.ParsePubKey(order.Listings[0].Listing.VendorID.Pubkeys.Escrow)
	if err != nil {
		return err
	}
	vendorKey, err := GenerateEscrowPublicKey(vendorEscrowPubkey, chaincode)
	if err != nil {
		return err
	}
	buyerEscrowPubkey, err := btcec.ParsePubKey(order.BuyerID.Pubkeys.Escrow)
	if err != nil {
		return err
	}
	buyerKey, err := GenerateEscrowPublicKey(buyerEscrowPubkey, chaincode)
	if err != nil {
		return err
	}

	if paymentSent.Method == pb.PaymentSent_MODERATED {
		return validateBTCEscrowPayment(paymentSent, wal, chaincode, vendorKey, buyerKey, escrowTimeoutHours, true)
	} else if paymentSent.Method == pb.PaymentSent_CANCELABLE {
		return validateBTCEscrowPayment(paymentSent, wal, chaincode, vendorKey, buyerKey, escrowTimeoutHours, false)
	} else if paymentSent.Method != pb.PaymentSent_DIRECT {
		return errors.New("invalid payment method")
	}

	return nil
}

var ErrPaymentAmountInsufficient = errors.New("payment amount is less than order amount")

func validateEscrowPayment(order *pb.OrderOpen, paymentSent *pb.PaymentSent, wal iwallet.Wallet) error {
	if order.Listings[0].Listing.Metadata.ContractType == pb.Listing_Metadata_RWA_TOKEN {
		return nil
	}

	if err := ValidatePaymentAmount(order.Amount, paymentSent.Amount); err != nil {
		return err
	}

	escrowInfo, err := GetOrderEscrowInfo(order, paymentSent, wal.IsTestnet())
	if err != nil {
		return err
	}
	escrowWallet := wal.(iwallet.EscrowProcessor)

	address, err := escrowWallet.CreateEscrowAddress(escrowInfo)
	if err != nil {
		return err
	}

	if paymentSent.ToAddress != address.String() {
		return errors.New("invalid escrow payment address")
	}

	return nil
}

func ValidatePaymentAmount(orderAmount, paidAmount string) error {
	expected, ok := new(big.Int).SetString(orderAmount, 10)
	if !ok || expected.Sign() <= 0 {
		return fmt.Errorf("invalid order amount: %q", orderAmount)
	}
	paid, ok := new(big.Int).SetString(paidAmount, 10)
	if !ok || paid.Sign() < 0 {
		return fmt.Errorf("invalid payment amount: %q", paidAmount)
	}
	if paid.Cmp(expected) < 0 {
		return fmt.Errorf("%w: paid %s < expected %s", ErrPaymentAmountInsufficient, paidAmount, orderAmount)
	}
	return nil
}

// validateEscrowPayment 验证托管支付
func validateBTCEscrowPayment(paymentSent *pb.PaymentSent, wal iwallet.Wallet, chaincode []byte,
	vendorKey, buyerKey *btcec.PublicKey, escrowTimeoutHours uint32, isModerated bool) error {
	var (
		address iwallet.Address
		script  []byte
		err     error
	)

	if isModerated {
		_, err := peer.Decode(paymentSent.Moderator)
		if err != nil {
			return errors.New("invalid moderator selection")
		}

		moderatorEscrowPubkeyBytes, err := hex.DecodeString(paymentSent.ModeratorAddress)
		if err != nil {
			return fmt.Errorf("decode moderator pubkey: %s", err.Error())
		}
		moderatorKey, err := btcec.ParsePubKey(moderatorEscrowPubkeyBytes)
		if err != nil {
			return fmt.Errorf("parse moderator key failed, %v", err)
		}

		moderatorKey, err = GenerateEscrowPublicKey(moderatorKey, chaincode)
		if err != nil {
			return err
		}

		escrowTimeoutWallet, walletSupportsEscrowTimeout := wal.(iwallet.UTXOEscrowWithTimeout)
		if !walletSupportsEscrowTimeout {
			escrowTimeoutHours = 0
		}

		if escrowTimeoutHours > 0 {
			timeout := time.Hour * time.Duration(escrowTimeoutHours)
			address, script, err = escrowTimeoutWallet.CreateMultisigWithTimeout([]btcec.PublicKey{*buyerKey, *vendorKey, *moderatorKey}, chaincode, 2, timeout, *vendorKey)
			if err != nil {
				return err
			}
		} else {
			escrowWallet, ok := wal.(iwallet.UTXOEscrow)
			if !ok {
				return errors.New("wallet does not support escrow")
			}
			address, script, err = escrowWallet.CreateMultisigAddress([]btcec.PublicKey{*buyerKey, *vendorKey, *moderatorKey}, chaincode, 2)
			if err != nil {
				return err
			}
		}
	} else {
		escrowWallet, ok := wal.(iwallet.UTXOEscrow)
		if !ok {
			return errors.New("wallet does not support escrow")
		}
		address, script, err = escrowWallet.CreateMultisigAddress([]btcec.PublicKey{*buyerKey, *vendorKey}, chaincode, 1)
		if err != nil {
			return err
		}
	}

	if paymentSent.ToAddress != address.String() {
		return errors.New("invalid escrow payment address")
	}
	if paymentSent.Script != hex.EncodeToString(script) {
		return errors.New("invalid escrow payment script")
	}

	// Note: EscrowReleaseFee validation removed - external wallets handle fee estimation
	// and actual release will re-estimate fees based on current network conditions
	return nil
}

// validateBigString validates that the string is a base10 big number.
func validateBigString(s string) bool {
	_, ok := new(big.Int).SetString(s, 10)
	return ok
}
