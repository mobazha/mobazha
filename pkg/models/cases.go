package models

import (
	"encoding/json"
	"errors"
	"time"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

type Case struct {
	ID OrderID `gorm:"primaryKey"`

	SerializedBuyerContract  []byte
	SerializedVendorContract []byte

	BuyerPayoutAddress  string
	VendorPayoutAddress string

	SerializedBuyerValidationErrors  []byte
	SerializedVendorValidationErrors []byte

	SerializedDisputeOpen  []byte
	SerializedDisputeClose []byte

	ParkedUpdate        []byte
	ParkedPayoutAddress string

	State OrderState

	Read               bool
	UnreadChatMessages int
	CreatedAt          time.Time
}

func (c *Case) BeforeSave(tx *gorm.DB) (err error) {
	c.State = c.GetState()

	tx.Statement.SetColumn("State", c.State)
	return nil
}

func (c *Case) DisuteOpenMessage() (*pb.DisputeOpen, error) {
	if len(c.SerializedDisputeOpen) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	disputeOpen := new(pb.DisputeOpen)
	if err := unmarshaler.Unmarshal(c.SerializedDisputeOpen, disputeOpen); err != nil {
		return nil, err
	}
	return disputeOpen, nil
}

func (c *Case) DisuteCloseMessage() (*pb.DisputeClose, error) {
	if len(c.SerializedDisputeClose) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	disputeClose := new(pb.DisputeClose)
	if err := unmarshaler.Unmarshal(c.SerializedDisputeClose, disputeClose); err != nil {
		return nil, err
	}
	return disputeClose, nil
}

func (c *Case) BuyerContract() (*pb.Contract, error) {
	if len(c.SerializedBuyerContract) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	buyerContract := new(pb.Contract)
	if err := proto.Unmarshal(c.SerializedBuyerContract, buyerContract); err != nil {
		return nil, err
	}
	return buyerContract, nil
}

func (c *Case) VendorContract() (*pb.Contract, error) {
	if len(c.SerializedVendorContract) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	vendorContract := new(pb.Contract)
	if err := proto.Unmarshal(c.SerializedVendorContract, vendorContract); err != nil {
		return nil, err
	}
	return vendorContract, nil
}

func (c *Case) OpenedBy() (pb.DisputeOpen_Party, error) {
	do, err := c.DisuteOpenMessage()
	if err != nil {
		return pb.DisputeOpen_BUYER, err
	}
	return do.OpenedBy, nil
}

func (c *Case) BuyerValidationErrors() ([]string, error) {
	if len(c.SerializedBuyerValidationErrors) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	var validationErrors []string
	if err := json.Unmarshal(c.SerializedBuyerValidationErrors, &validationErrors); err != nil {
		return nil, err
	}
	return validationErrors, nil
}

func (c *Case) VendorValidationErrors() ([]string, error) {
	if len(c.SerializedVendorValidationErrors) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	var validationErrors []string
	if err := json.Unmarshal(c.SerializedVendorValidationErrors, &validationErrors); err != nil {
		return nil, err
	}
	return validationErrors, nil
}

// GetState returns the order state
func (c *Case) GetState() OrderState {
	cloneCase := *c

	if cloneCase.SerializedDisputeOpen != nil {
		if cloneCase.SerializedDisputeClose == nil {
			return OrderState_DISPUTED
		} else if cloneCase.SerializedDisputeClose != nil {
			return OrderState_DECIDED
		}
	}

	return OrderState_PROCESSING_ERROR
}

func (c *Case) PutDisputeOpen(disputeOpen *pb.DisputeOpen) error {
	if disputeOpen.OpenedBy == pb.DisputeOpen_BUYER {
		c.SerializedBuyerContract = disputeOpen.Contract
		c.BuyerPayoutAddress = disputeOpen.PayoutAddress
	} else {
		c.SerializedVendorContract = disputeOpen.Contract
		c.VendorPayoutAddress = disputeOpen.PayoutAddress
	}

	disputeOpen.Contract = nil
	out := marshaler.Format(disputeOpen)

	c.SerializedDisputeOpen = []byte(out)

	if c.ParkedUpdate != nil {
		if disputeOpen.OpenedBy == pb.DisputeOpen_BUYER {
			c.SerializedVendorContract = c.ParkedUpdate
			c.VendorPayoutAddress = c.ParkedPayoutAddress
		} else {
			c.SerializedBuyerContract = c.ParkedUpdate
			c.BuyerPayoutAddress = c.ParkedPayoutAddress
		}
		c.ParkedUpdate = nil
		c.ParkedPayoutAddress = ""
	}
	return nil
}

func (c *Case) PutDisputeUpdate(disputeUpdate *pb.DisputeUpdate) error {
	disputeOpen, err := c.DisuteOpenMessage()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return err
	}

	if errors.Is(err, ErrMessageDoesNotExist) {
		c.ParkedUpdate = disputeUpdate.Contract
		c.ParkedPayoutAddress = disputeUpdate.PayoutAddress
		return nil
	}

	if disputeOpen.OpenedBy == pb.DisputeOpen_BUYER {
		if c.SerializedVendorContract != nil {
			return errors.New("DISPUTE_UPDATE already exists")
		}
		c.SerializedVendorContract = disputeUpdate.Contract
		c.VendorPayoutAddress = disputeUpdate.PayoutAddress
	} else {
		if c.SerializedBuyerContract != nil {
			return errors.New("DISPUTE_UPDATE already exists")
		}
		c.SerializedBuyerContract = disputeUpdate.Contract
		c.BuyerPayoutAddress = disputeUpdate.PayoutAddress
	}

	return nil
}

func (c *Case) PutValidationErrors(validationErrors []error, role OrderRole) error {
	errStrs := make([]string, 0, len(validationErrors))
	for _, err := range validationErrors {
		errStrs = append(errStrs, err.Error())
	}

	out, err := json.MarshalIndent(errStrs, "", "    ")
	if err != nil {
		return err
	}
	if role == RoleVendor {
		c.SerializedVendorValidationErrors = out
	} else {
		c.SerializedBuyerValidationErrors = out
	}
	return nil
}

func (c *Case) PutDisputeClose(disputeClose *pb.DisputeClose) error {
	out := marshaler.Format(disputeClose)

	c.SerializedDisputeClose = []byte(out)

	return nil
}

// ResolutionPaymentContract returns the preferred contract to be used when resolving
// a pending Case based on the provided PayoutRatio
func (c *Case) ResolutionPaymentContract(ratio PayoutRatio) (*pb.Contract, error) {
	preferredContract := c.SerializedBuyerContract
	if ratio.VendorMajority() {
		preferredContract = c.SerializedVendorContract
	}

	if len(preferredContract) == 0 {
		return nil, ErrMessageDoesNotExist
	}

	var contract pb.Contract
	if err := proto.Unmarshal(preferredContract, &contract); err != nil {
		return nil, err
	}
	return &contract, nil
}

// MarshalBinary returns a serialized protobuf format.
func (c *Case) MarshalBinary() ([]byte, error) {
	aCase, err := c.toProtobuf()
	if err != nil {
		return nil, err
	}

	return proto.Marshal(aCase)
}

// MarshalJSON provides custom JSON marshalling for the case model. Since this method is primarily
// used to return data to the API, this is the appropriate place to normalize the data to the format
// the API is expecting.
func (c *Case) MarshalJSON() ([]byte, error) {
	aCase, err := c.toProtobuf()
	if err != nil {
		return nil, err
	}

	out := marshaler.Format(aCase)

	return []byte(out), nil
}

func (c *Case) toProtobuf() (*pb.Case, error) {
	c0 := pb.Case{
		OrderID: c.ID.String(),
	}

	var err error
	c0.BuyerContract, err = c.BuyerContract()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}

	c0.VendorContract, err = c.VendorContract()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}

	c0.BuyerContractValidationErrors, err = c.BuyerValidationErrors()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}

	c0.VendorContractValidationErrors, err = c.VendorValidationErrors()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}

	c0.DisputeOpen, err = c.DisuteOpenMessage()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}

	c0.DisputeClose, err = c.DisuteCloseMessage()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}

	c0.State = c.GetState().String()
	c0.Read = c.Read
	c0.UnreadChatMessages = uint64(c.UnreadChatMessages)

	return &c0, nil
}
