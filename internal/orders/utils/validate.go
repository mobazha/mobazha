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

	if wal.CoinCategory() == iwallet.CoinCategorySolana {
		return validateSolanaPayment(order, paymentSent, wal)
	} else if wal.CoinCategory() == iwallet.CoinCategoryEthereum {
		return validateETHLikePayment(order, paymentSent, chaincode, wal, escrowTimeoutHours)
	} else if wal.CoinCategory() == iwallet.CoinCategoryStripe {
		return validateStripePayment(order, paymentSent, chaincode, wal, escrowTimeoutHours)
	} else {
		return validateBTCLikePayment(order, paymentSent, chaincode, wal, escrowTimeoutHours)
	}
}

// validateETHLikePayment 验证ETH类支付
func validateETHLikePayment(order *pb.OrderOpen, paymentSent *pb.PaymentSent, chaincode []byte, wal iwallet.Wallet, escrowTimeoutHours uint32) error {
	vendorKey, err := btcec.ParsePubKey(order.Listings[0].Listing.VendorID.Pubkeys.Eth)
	if err != nil {
		return fmt.Errorf("generate vendor pub key failed, %s", err)
	}

	buyerKey, err := btcec.ParsePubKey(order.BuyerID.Pubkeys.Eth)
	if err != nil {
		return fmt.Errorf("get buyer pub key failed, %s", err)
	}

	if paymentSent.Method == pb.PaymentSent_MODERATED {
		return validateEscrowPayment(paymentSent, wal, chaincode, vendorKey, buyerKey, escrowTimeoutHours, true, true)
	} else if paymentSent.Method == pb.PaymentSent_CANCELABLE {
		return validateEscrowPayment(paymentSent, wal, chaincode, vendorKey, buyerKey, escrowTimeoutHours, true, false)
	} else if paymentSent.Method != pb.PaymentSent_DIRECT {
		return errors.New("invalid payment method")
	}

	return nil
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
		return validateEscrowPayment(paymentSent, wal, chaincode, vendorKey, buyerKey, escrowTimeoutHours, false, true)
	} else if paymentSent.Method == pb.PaymentSent_CANCELABLE {
		return validateEscrowPayment(paymentSent, wal, chaincode, vendorKey, buyerKey, escrowTimeoutHours, false, false)
	} else if paymentSent.Method != pb.PaymentSent_DIRECT {
		return errors.New("invalid payment method")
	}

	return nil
}

func validateSolanaPayment(order *pb.OrderOpen, paymentSent *pb.PaymentSent, wal iwallet.Wallet) error {
	escrowInfo, err := GetOrderEscrowInfo(order, paymentSent)
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

	// if err := validateEscrowReleaseFee(paymentSent); err != nil {
	// 	return err
	// }
	return nil
}

func validateStripePayment(order *pb.OrderOpen, paymentSent *pb.PaymentSent, chaincode []byte, wal iwallet.Wallet, escrowTimeoutHours uint32) error {
	return nil
}

// validateEscrowPayment 验证托管支付
func validateEscrowPayment(paymentSent *pb.PaymentSent, wal iwallet.Wallet, chaincode []byte,
	vendorKey, buyerKey *btcec.PublicKey, escrowTimeoutHours uint32, isETHLike bool, isModerated bool) error {
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

		if !isETHLike {
			moderatorKey, err = GenerateEscrowPublicKey(moderatorKey, chaincode)
			if err != nil {
				return err
			}
		}

		escrowTimeoutWallet, walletSupportsEscrowTimeout := wal.(iwallet.EscrowWithTimeout)
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
			escrowWallet, ok := wal.(iwallet.Escrow)
			if !ok {
				return errors.New("wallet does not support escrow")
			}
			address, script, err = escrowWallet.CreateMultisigAddress([]btcec.PublicKey{*buyerKey, *vendorKey, *moderatorKey}, chaincode, 2)
			if err != nil {
				return err
			}
		}
	} else {
		escrowWallet, ok := wal.(iwallet.Escrow)
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

	if err := validateEscrowReleaseFee(paymentSent); err != nil {
		return err
	}
	return nil
}

// validateEscrowReleaseFee 验证托管释放费用
func validateEscrowReleaseFee(paymentSent *pb.PaymentSent) error {
	if paymentSent.EscrowReleaseFee == "" {
		return errors.New("escrow release fee is empty")
	}
	if ok := validateBigString(paymentSent.EscrowReleaseFee); !ok {
		return errors.New("escrow release fee not valid")
	}
	return nil
}

// validateBigString validates that the string is a base10 big number.
func validateBigString(s string) bool {
	_, ok := new(big.Int).SetString(s, 10)
	return ok
}
