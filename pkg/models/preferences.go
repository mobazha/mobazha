package models

import (
	"encoding/json"
	"strings"

	peer "github.com/libp2p/go-libp2p/core/peer"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

// UserPreferences are set by the client and persisted in the database.
type UserPreferences struct {
	ID                  int     `json:"-" gorm:"primaryKey"`
	UserAgent           string  `json:"userAgent"`
	PaymentDataInQR     bool    `json:"paymentDataInQR"`
	ShowNotifications   bool    `json:"showNotifications"`
	ShowNsfw            bool    `json:"showNsfw"`
	ShippingAddresses   []byte  `json:"shippingAddresses"` // Self receiving address
	ShippingOptions     []byte  `json:"shippingOptions"`   // As vendor, for the buyer to delivery choose
	LocalCurrency       string  `json:"localCurrency"`
	Country             string  `json:"country"`
	TermsAndConditions  string  `json:"termsAndConditions"`
	RefundPolicy        string  `json:"refundPolicy"`
	Blocked             []byte  `json:"blockedNodes"`
	Mods                []byte  `json:"storeModerators"`
	ExtPaymentAddresses []byte  `json:"externalPaymentAddresses"`
	MisPaymentBuffer    float32 `json:"mispaymentBuffer"`
	AutoConfirm         bool    `json:"autoConfirm"`
	EmailNotifications  string  `json:"emailNotifications"`
	PrefCurrencies      []byte  `json:"preferredCurrencies"`
	ChannelSubs         []byte  `json:"channelSubscriptions"`
}

type AddressEnablement struct {
	Address string `json:"address"`
	Enable  bool   `json:"enable"`
}

type shippingAddress struct {
	Name           string `json:"name"`
	Company        string `json:"company"`
	AddressLineOne string `json:"addressLineOne"`
	AddressLineTwo string `json:"addressLineTwo"`
	City           string `json:"city"`
	State          string `json:"state"`
	Country        string `json:"country"`
	PostalCode     string `json:"postalCode"`
	AddressNotes   string `json:"addressNotes"`
}

type ShippingOption_Service struct {
	Name              string `json:"name"`
	EstimatedDelivery string `json:"estimatedDelivery"`
	StartWeight       uint32 `json:"startWeight"`
	EndWeight         uint32 `json:"endWeight"`
	FirstWeight       uint32 `json:"firstWeight"`
	FirstFreight      string `json:"firstFreight"`
	RenewalUnitWeight uint32 `json:"renewalUnitWeight"`
	RenewalUnitPrice  string `json:"renewalUnitPrice"`
	RegistrationFee   string `json:"registrationFee"`
}

type ShippingOption struct {
	ID          int                       `json:"id" gorm:"primaryKey"`
	Name        string                    `json:"name"`
	Type        string                    `json:"type"`
	Currency    string                    `json:"currency"`
	ServiceType string                    `json:"serviceType"`
	Regions     []string                  `json:"regions"`
	Services    []*ShippingOption_Service `json:"services"`
}

func ConvertShippingOption(option ShippingOption) *pb.Listing_ShippingOption {
	shippingOption := &pb.Listing_ShippingOption{
		OptionID:    uint32(option.ID),
		Name:        option.Name,
		Type:        pb.Listing_ShippingOption_ShippingType(pb.Listing_ShippingOption_ShippingType_value[option.Type]),
		Currency:    option.Currency,
		ServiceType: pb.Listing_ShippingOption_ServiceType(pb.Listing_ShippingOption_ServiceType_value[option.ServiceType]),
	}

	for _, region := range option.Regions {
		shippingOption.Regions = append(shippingOption.Regions, strings.ToUpper(region))
	}

	for _, service := range option.Services {
		shippingOption.Services = append(shippingOption.Services, &pb.Listing_ShippingOption_Service{
			Name:              service.Name,
			EstimatedDelivery: service.EstimatedDelivery,
			StartWeight:       service.StartWeight,
			EndWeight:         service.EndWeight,
			FirstWeight:       service.FirstWeight,
			FirstFreight:      service.FirstFreight,
			RenewalUnitWeight: service.RenewalUnitWeight,
			RenewalUnitPrice:  service.RenewalUnitPrice,
			RegistrationFee:   service.RegistrationFee,
		})
	}

	return shippingOption
}

func ConvertShippingOptions(options []ShippingOption) []*pb.Listing_ShippingOption {
	shippingOptions := make([]*pb.Listing_ShippingOption, 0)
	for _, option := range options {
		shippingOptions = append(shippingOptions, ConvertShippingOption(option))
	}
	return shippingOptions
}

type prefsJSON struct {
	UserAgent                string                       `json:"userAgent"`
	PaymentDataInQR          bool                         `json:"paymentDataInQR"`
	ShowNotifications        bool                         `json:"showNotifications"`
	ShowNsfw                 bool                         `json:"showNsfw"`
	ShippingAddresses        []shippingAddress            `json:"shippingAddresses"`
	ShippingOptions          []ShippingOption             `json:"shippingOptions"`
	LocalCurrency            string                       `json:"localCurrency"`
	Country                  string                       `json:"country"`
	TermsAndConditions       string                       `json:"termsAndConditions"`
	RefundPolicy             string                       `json:"refundPolicy"`
	BlockedNodes             []string                     `json:"blockedNodes"`
	StoreModerators          []string                     `json:"storeModerators"`
	ExternalPaymentAddresses map[string]AddressEnablement `json:"externalPaymentAddresses"`
	MisPaymentBuffer         float32                      `json:"mispaymentBuffer"`
	AutoConfirm              bool                         `json:"autoConfirm"`
	EmailNotifications       string                       `json:"emailNotifications"`
	PreferredCurrencies      []string                     `json:"preferredCurrencies"`
	ChannelSubscriptions     []string                     `json:"channelSubscriptions"`
}

func (prefs *UserPreferences) GetShippingAddresses() ([]shippingAddress, error) {
	var addresses []shippingAddress
	if prefs.ShippingAddresses != nil {
		if err := json.Unmarshal(prefs.ShippingAddresses, &addresses); err != nil {
			return nil, err
		}
	}
	return addresses, nil
}

func (prefs *UserPreferences) GetShippingOptions() ([]ShippingOption, error) {
	var options []ShippingOption
	if prefs.ShippingOptions != nil {
		if err := json.Unmarshal(prefs.ShippingOptions, &options); err != nil {
			return nil, err
		}
	}
	return options, nil
}

// StoreModerators returns the moderator peer IDs.
func (prefs *UserPreferences) StoreModerators() ([]peer.ID, error) {
	var peerIDStrs []string
	if prefs.Mods != nil {
		if err := json.Unmarshal(prefs.Mods, &peerIDStrs); err != nil {
			return nil, err
		}
	}
	ret := make([]peer.ID, 0, len(peerIDStrs))
	for _, s := range peerIDStrs {
		pid, err := peer.Decode(s)
		if err != nil {
			return nil, err
		}
		ret = append(ret, pid)
	}
	return ret, nil
}

func (prefs *UserPreferences) ExternalPaymentAddresses() (map[string]AddressEnablement, error) {
	var enablements map[string]AddressEnablement
	if prefs.ExtPaymentAddresses != nil {
		if err := json.Unmarshal(prefs.ExtPaymentAddresses, &enablements); err != nil {
			return nil, err
		}
	}
	return enablements, nil
}

// BlockedNodes returns the blocked peer IDs.
func (prefs *UserPreferences) BlockedNodes() ([]peer.ID, error) {
	var peerIDStrs []string
	if prefs.Blocked != nil {
		if err := json.Unmarshal(prefs.Blocked, &peerIDStrs); err != nil {
			return nil, err
		}
	}
	ret := make([]peer.ID, 0, len(peerIDStrs))
	for _, s := range peerIDStrs {
		pid, err := peer.Decode(s)
		if err != nil {
			return nil, err
		}
		ret = append(ret, pid)
	}
	return ret, nil
}

func (prefs *UserPreferences) AddBlockedNode(peerID string) (bool, error) {
	_, err := peer.Decode(peerID)
	if err != nil {
		return false, err
	}

	var peerIDStrs []string
	if prefs.Blocked != nil {
		if err := json.Unmarshal(prefs.Blocked, &peerIDStrs); err != nil {
			return false, err
		}
	}

	var nodes []string
	for _, pid := range peerIDStrs {
		if pid == peerID {
			log.Debugf("The node has already been blocked, peer id: %s", peerID)
			return false, nil
		}
		nodes = append(nodes, pid)
	}
	nodes = append(nodes, peerID)

	blockedNodes, err := json.Marshal(nodes)
	if err != nil {
		return false, err
	}
	log.Debugf("Add the blocked node, peer id: %s", peerID)
	prefs.Blocked = blockedNodes
	return true, nil
}

func (prefs *UserPreferences) RemoveBlockedNode(peerID string) (bool, error) {
	var peerIDStrs []string
	if prefs.Blocked != nil {
		if err := json.Unmarshal(prefs.Blocked, &peerIDStrs); err != nil {
			return false, err
		}
	}

	var nodes []string
	found := false
	for _, pid := range peerIDStrs {
		if pid == peerID {
			found = true
			continue
		}
		nodes = append(nodes, pid)
	}
	if !found {
		log.Debugf("Skip, the node is not in blocked list, peer id: ", peerID)
		return false, nil
	}

	blockedNodes, err := json.Marshal(nodes)
	if err != nil {
		return false, err
	}
	log.Debugf("Remove the blocked node, peer id: %s", peerID)
	prefs.Blocked = blockedNodes
	return true, nil
}

// PreferredCurrencies returns the preferred currencies for the node.
func (prefs *UserPreferences) PreferredCurrencies() ([]string, error) {
	var prefCurrencies []string
	if prefs.PrefCurrencies != nil {
		if err := json.Unmarshal(prefs.PrefCurrencies, &prefCurrencies); err != nil {
			return nil, err
		}
	}
	return prefCurrencies, nil
}

// ChannelSubscriptions returns the channels this node is subscribed to.
func (prefs *UserPreferences) ChannelSubscriptions() ([]string, error) {
	var subs []string
	if prefs.ChannelSubs != nil {
		if err := json.Unmarshal(prefs.ChannelSubs, &subs); err != nil {
			return nil, err
		}
	}
	return subs, nil
}

func (prefs *UserPreferences) MarshalJSON() ([]byte, error) {
	var c0 prefsJSON

	c0.UserAgent = prefs.UserAgent
	c0.PaymentDataInQR = prefs.PaymentDataInQR
	c0.ShowNotifications = prefs.ShowNotifications
	c0.ShowNsfw = prefs.ShowNsfw
	c0.ShippingAddresses, _ = prefs.GetShippingAddresses()
	c0.ShippingOptions, _ = prefs.GetShippingOptions()
	c0.LocalCurrency = prefs.LocalCurrency
	c0.Country = prefs.Country
	c0.TermsAndConditions = prefs.TermsAndConditions
	c0.RefundPolicy = prefs.RefundPolicy
	blocked, _ := prefs.BlockedNodes()
	for _, blockedNode := range blocked {
		c0.BlockedNodes = append(c0.BlockedNodes, blockedNode.String())
	}
	mods, _ := prefs.StoreModerators()
	for _, mod := range mods {
		c0.StoreModerators = append(c0.StoreModerators, mod.String())
	}
	extAddresses, _ := prefs.ExternalPaymentAddresses()
	c0.ExternalPaymentAddresses = extAddresses
	c0.MisPaymentBuffer = prefs.MisPaymentBuffer
	c0.AutoConfirm = prefs.AutoConfirm
	c0.EmailNotifications = prefs.EmailNotifications
	c0.PreferredCurrencies, _ = prefs.PreferredCurrencies()
	c0.ChannelSubscriptions, _ = prefs.ChannelSubscriptions()

	return json.Marshal(c0)
}

// UnmarshalJSON unmarshals the JSON object into a UserPreferences object.
func (prefs *UserPreferences) UnmarshalJSON(b []byte) error {
	var c0 prefsJSON

	err := json.Unmarshal(b, &c0)
	if err == nil {
		shippingAddrs, err := json.Marshal(c0.ShippingAddresses)
		if err != nil {
			return err
		}
		shippingOptions, err := json.Marshal(c0.ShippingOptions)
		if err != nil {
			return err
		}
		blockedNodes, err := json.Marshal(c0.BlockedNodes)
		if err != nil {
			return err
		}
		storeModerators, err := json.Marshal(c0.StoreModerators)
		if err != nil {
			return err
		}
		extPaymentAddress, err := json.Marshal(c0.ExternalPaymentAddresses)
		if err != nil {
			return err
		}
		preferredCurrencies, err := json.Marshal(c0.PreferredCurrencies)
		if err != nil {
			return err
		}
		channelSubscriptions, err := json.Marshal(c0.ChannelSubscriptions)
		if err != nil {
			return err
		}

		prefs.PaymentDataInQR = c0.PaymentDataInQR
		prefs.ShowNotifications = c0.ShowNotifications
		prefs.ShowNsfw = c0.ShowNsfw
		prefs.ShippingAddresses = shippingAddrs
		prefs.ShippingOptions = shippingOptions
		prefs.LocalCurrency = c0.LocalCurrency
		prefs.Country = c0.Country
		prefs.TermsAndConditions = c0.TermsAndConditions
		prefs.RefundPolicy = c0.RefundPolicy
		prefs.Blocked = blockedNodes
		prefs.Mods = storeModerators
		prefs.ExtPaymentAddresses = extPaymentAddress
		prefs.MisPaymentBuffer = c0.MisPaymentBuffer
		prefs.AutoConfirm = c0.AutoConfirm
		prefs.EmailNotifications = c0.EmailNotifications
		prefs.PrefCurrencies = preferredCurrencies
		prefs.ChannelSubs = channelSubscriptions
	}

	return err
}
