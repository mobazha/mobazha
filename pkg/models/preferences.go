package models

import (
	"encoding/json"
	"fmt"
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
	ShippingAddresses   []byte  `json:"shippingAddresses"`  // Self receiving address
	ShippingOptions     []byte  `json:"shippingOptions"`    // 旧版：作为 vendor，供买家选择的运费选项（向后兼容）
	ShippingProfiles    []byte  `json:"shippingProfiles"`   // 新版：配送档案列表（Shopify 模式）
	ShippingLocations   []byte  `json:"shippingLocations"`  // v2：发货地点列表
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
	ID                    int                        `json:"id" gorm:"primaryKey"`
	Name                  string                     `json:"name"`
	Type                  string                     `json:"type"`
	Currency              string                     `json:"currency"`
	ServiceType           string                     `json:"serviceType"`
	Regions               []string                   `json:"regions"`
	Services              []*ShippingOption_Service  `json:"services"`
	FreeShippingThreshold *FreeShippingThreshold     `json:"freeShippingThreshold,omitempty"`
}

// FreeShippingThreshold 满额免邮配置
type FreeShippingThreshold struct {
	Enabled   bool   `json:"enabled"`
	MinAmount string `json:"minAmount"`
}

// ============== Shopify 风格配送系统 (v2) ==============

// ShippingLocation 发货地点
type ShippingLocation struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Address   string `json:"address,omitempty"`
	IsDefault bool   `json:"isDefault"`
}

// RateCondition 费率条件（可选）
type RateCondition struct {
	Type     string `json:"type"`     // "weight" | "price"
	MinValue uint32 `json:"minValue"` // 最小值（重量为克，价格为最小单位）
	MaxValue uint32 `json:"maxValue"` // 最大值（0 表示无上限）
}

// ShippingRate 配送费率（对应 Shopify 的 Rate）
type ShippingRate struct {
	ID                    string                 `json:"id"`
	Name                  string                 `json:"name"`                            // 如 "标准配送"、"快递"
	Price                 string                 `json:"price"`                           // 支持加密货币精度
	Currency              string                 `json:"currency"`
	EstimatedDelivery     string                 `json:"estimatedDelivery"`
	Condition             *RateCondition         `json:"condition,omitempty"`             // 可选条件
	FreeShippingThreshold *FreeShippingThreshold `json:"freeShippingThreshold,omitempty"` // 满额免邮
}

// ShippingZone 配送区域（对应 Shopify 的 Delivery Zone）
type ShippingZone struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`    // 如 "全球"、"亚洲"、"北美"
	Regions []string        `json:"regions"` // ISO 国家代码
	Rates   []*ShippingRate `json:"rates"`
}

// LocationGroup 发货地点组（对应 Shopify 的 Location Group）
type LocationGroup struct {
	ID          string          `json:"id"`
	LocationIDs []string        `json:"locationIds"` // 关联的发货地点
	Zones       []*ShippingZone `json:"zones"`
}

// ShippingProfile 配送档案 - 允许卖家创建多个配送方案（Shopify 风格）
// 始终使用 LocationGroups，每个 LocationGroup 包含独立的配送区域
// 单仓库卖家自动创建一个默认 LocationGroup（LocationIDs 留空 = 全局适用）
type ShippingProfile struct {
	ProfileID      string           `json:"profileId"`
	Name           string           `json:"name"`
	IsDefault      bool             `json:"isDefault"`
	LocationGroups []*LocationGroup  `json:"locationGroups,omitempty"` // 发货地点组（至少一个）
	CreatedAt      string           `json:"createdAt,omitempty"`
	UpdatedAt      string           `json:"updatedAt,omitempty"`
}

// GetAllZones 获取所有 LocationGroup 中的配送区域
func (p *ShippingProfile) GetAllZones() []*ShippingZone {
	var zones []*ShippingZone
	for _, lg := range p.LocationGroups {
		zones = append(zones, lg.Zones...)
	}
	return zones
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

	// 处理满额免邮配置
	if option.FreeShippingThreshold != nil {
		shippingOption.FreeShippingThreshold = &pb.Listing_ShippingOption_FreeShippingThreshold{
			Enabled:   option.FreeShippingThreshold.Enabled,
			MinAmount: option.FreeShippingThreshold.MinAmount,
		}
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

// ConvertShippingOptionFromPointer 从指针类型的 ShippingOption 转换
func ConvertShippingOptionFromPointer(option *ShippingOption) *pb.Listing_ShippingOption {
	if option == nil {
		return nil
	}
	return ConvertShippingOption(*option)
}

// ConvertShippingOptionsFromPointers 从指针数组转换
func ConvertShippingOptionsFromPointers(options []*ShippingOption) []*pb.Listing_ShippingOption {
	shippingOptions := make([]*pb.Listing_ShippingOption, 0, len(options))
	for _, option := range options {
		if option != nil {
			shippingOptions = append(shippingOptions, ConvertShippingOption(*option))
		}
	}
	return shippingOptions
}

// ConvertShippingProfileToProto 将 JSON 格式的 ShippingProfile 转换为 protobuf 格式
func ConvertShippingProfileToProto(profile *ShippingProfile) *pb.ShippingProfile {
	if profile == nil {
		return nil
	}

	pbProfile := &pb.ShippingProfile{
		ProfileID: profile.ProfileID,
		Name:      profile.Name,
		IsDefault: profile.IsDefault,
	}

	// 转换 LocationGroups
	for _, lg := range profile.LocationGroups {
		if lg == nil {
			continue
		}
		pbLG := &pb.LocationGroup{
			Id:          lg.ID,
			LocationIds: lg.LocationIDs,
		}
		for _, zone := range lg.Zones {
			if zone != nil {
				pbLG.Zones = append(pbLG.Zones, convertZoneToProto(zone))
			}
		}
		pbProfile.LocationGroups = append(pbProfile.LocationGroups, pbLG)
	}

	return pbProfile
}

// convertZoneToProto 将 ShippingZone 转换为 protobuf 格式
func convertZoneToProto(zone *ShippingZone) *pb.ShippingZone {
	if zone == nil {
		return nil
	}

	pbZone := &pb.ShippingZone{
		Id:      zone.ID,
		Name:    zone.Name,
		Regions: zone.Regions,
	}

	for _, rate := range zone.Rates {
		if rate == nil {
			continue
		}
		pbRate := &pb.ShippingRate{
			Id:                rate.ID,
			Name:              rate.Name,
			Price:             rate.Price,
			Currency:          rate.Currency,
			EstimatedDelivery: rate.EstimatedDelivery,
		}

		// 转换条件
		if rate.Condition != nil {
			condType := pb.ShippingRate_RateCondition_NONE
			switch rate.Condition.Type {
			case "weight":
				condType = pb.ShippingRate_RateCondition_WEIGHT
			case "price":
				condType = pb.ShippingRate_RateCondition_PRICE
			}
			pbRate.Condition = &pb.ShippingRate_RateCondition{
				Type:     condType,
				MinValue: rate.Condition.MinValue,
				MaxValue: rate.Condition.MaxValue,
			}
		}

		// 转换满额免邮
		if rate.FreeShippingThreshold != nil {
			pbRate.FreeShippingThreshold = &pb.ShippingRate_FreeShippingThreshold{
				Enabled:   rate.FreeShippingThreshold.Enabled,
				MinAmount: rate.FreeShippingThreshold.MinAmount,
			}
		}

		pbZone.Rates = append(pbZone.Rates, pbRate)
	}

	return pbZone
}

// ConvertShippingLocationToProto 将 ShippingLocation 转换为 protobuf 格式
func ConvertShippingLocationToProto(location *ShippingLocation) *pb.ShippingLocation {
	if location == nil {
		return nil
	}
	return &pb.ShippingLocation{
		Id:        location.ID,
		Name:      location.Name,
		Address:   location.Address,
		IsDefault: location.IsDefault,
	}
}

type prefsJSON struct {
	UserAgent                string                       `json:"userAgent"`
	PaymentDataInQR          bool                         `json:"paymentDataInQR"`
	ShowNotifications        bool                         `json:"showNotifications"`
	ShowNsfw                 bool                         `json:"showNsfw"`
	ShippingAddresses        []shippingAddress            `json:"shippingAddresses"`
	ShippingOptions          []ShippingOption             `json:"shippingOptions"`
	ShippingProfiles         []*ShippingProfile           `json:"shippingProfiles,omitempty"`
	ShippingLocations        []*ShippingLocation          `json:"shippingLocations,omitempty"`
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

// GetShippingProfiles 获取配送档案列表
func (prefs *UserPreferences) GetShippingProfiles() ([]*ShippingProfile, error) {
	var profiles []*ShippingProfile
	if prefs.ShippingProfiles != nil {
		if err := json.Unmarshal(prefs.ShippingProfiles, &profiles); err != nil {
			return nil, err
		}
	}
	return profiles, nil
}

// GetShippingLocations 获取发货地点列表
func (prefs *UserPreferences) GetShippingLocations() ([]*ShippingLocation, error) {
	var locations []*ShippingLocation
	if prefs.ShippingLocations != nil {
		if err := json.Unmarshal(prefs.ShippingLocations, &locations); err != nil {
			return nil, err
		}
	}
	return locations, nil
}

// SetShippingLocations 设置发货地点列表
func (prefs *UserPreferences) SetShippingLocations(locations []*ShippingLocation) error {
	data, err := json.Marshal(locations)
	if err != nil {
		return err
	}
	prefs.ShippingLocations = data
	return nil
}

// GetDefaultShippingLocation 获取默认发货地点
func (prefs *UserPreferences) GetDefaultShippingLocation() (*ShippingLocation, error) {
	locations, err := prefs.GetShippingLocations()
	if err != nil {
		return nil, err
	}
	for _, loc := range locations {
		if loc.IsDefault {
			return loc, nil
		}
	}
	// 如果没有设置默认地点，返回第一个
	if len(locations) > 0 {
		return locations[0], nil
	}
	return nil, nil
}

// HasMultipleLocations 检查是否有多个发货地点
func (prefs *UserPreferences) HasMultipleLocations() bool {
	locations, err := prefs.GetShippingLocations()
	if err != nil {
		return false
	}
	return len(locations) > 1
}

// EnsureDefaultLocation 确保存在默认发货地点（用于迁移）
func (prefs *UserPreferences) EnsureDefaultLocation(locationID, locationName string) error {
	locations, err := prefs.GetShippingLocations()
	if err != nil {
		return err
	}
	if len(locations) > 0 {
		return nil // 已有发货地点
	}
	// 创建默认发货地点
	defaultLocation := &ShippingLocation{
		ID:        locationID,
		Name:      locationName,
		IsDefault: true,
	}
	return prefs.SetShippingLocations([]*ShippingLocation{defaultLocation})
}

// GetDefaultShippingProfile 获取默认配送档案
func (prefs *UserPreferences) GetDefaultShippingProfile() (*ShippingProfile, error) {
	profiles, err := prefs.GetShippingProfiles()
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if profile.IsDefault {
			return profile, nil
		}
	}
	// 如果没有设置默认档案，返回第一个
	if len(profiles) > 0 {
		return profiles[0], nil
	}
	return nil, nil
}

// GetShippingProfileByID 根据ID获取配送档案
func (prefs *UserPreferences) GetShippingProfileByID(profileID string) (*ShippingProfile, error) {
	profiles, err := prefs.GetShippingProfiles()
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if profile.ProfileID == profileID {
			return profile, nil
		}
	}
	return nil, nil
}

// SetShippingProfiles 设置配送档案列表
func (prefs *UserPreferences) SetShippingProfiles(profiles []*ShippingProfile) error {
	data, err := json.Marshal(profiles)
	if err != nil {
		return err
	}
	prefs.ShippingProfiles = data
	return nil
}

// HasShippingProfiles 检查是否有配送档案（用于判断是否使用新版配送系统）
func (prefs *UserPreferences) HasShippingProfiles() bool {
	profiles, err := prefs.GetShippingProfiles()
	if err != nil {
		return false
	}
	return len(profiles) > 0
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
	c0.ShippingProfiles, _ = prefs.GetShippingProfiles()
	c0.ShippingLocations, _ = prefs.GetShippingLocations()
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
		shippingProfiles, err := json.Marshal(c0.ShippingProfiles)
		if err != nil {
			return err
		}
		shippingLocations, err := json.Marshal(c0.ShippingLocations)
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
		prefs.ShippingProfiles = shippingProfiles
		prefs.ShippingLocations = shippingLocations
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

// NeedsMigrationFromLegacy 检查是否需要从最早版本的 ShippingOptions 迁移
func (prefs *UserPreferences) NeedsMigrationFromLegacy() bool {
	// 如果已有配送档案，不需要迁移
	if prefs.HasShippingProfiles() {
		return false
	}
	// 如果有旧版运费选项，需要迁移
	options, err := prefs.GetShippingOptions()
	if err != nil {
		return false
	}
	return len(options) > 0
}

// MigrateFromLegacyShippingOptions 将最早版本的 ShippingOptions 迁移到新的 Zones 格式
// profileID 是新创建的默认档案的 ID（由调用方提供，通常是 UUID）
// profileName 是默认档案的名称
// locationID 是默认发货地点的 ID
// locationName 是默认发货地点的名称
func (prefs *UserPreferences) MigrateFromLegacyShippingOptions(profileID, profileName, locationID, locationName string) error {
	if !prefs.NeedsMigrationFromLegacy() {
		return nil
	}

	options, err := prefs.GetShippingOptions()
	if err != nil {
		return err
	}

	if len(options) == 0 {
		return nil
	}

	// 将旧的 ShippingOptions 转换为新的 Zones 格式
	zones := make([]*ShippingZone, 0, len(options))
	for _, option := range options {
		zone := convertLegacyOptionToZone(&option)
		if zone != nil {
			zones = append(zones, zone)
		}
	}

	// 创建默认配送档案，将 zones 放入默认 LocationGroup
	defaultProfile := &ShippingProfile{
		ProfileID: profileID,
		Name:      profileName,
		IsDefault: true,
		LocationGroups: []*LocationGroup{
			{
				ID:    "default",
				Zones: zones,
			},
		},
	}

	// 保存配送档案
	if err := prefs.SetShippingProfiles([]*ShippingProfile{defaultProfile}); err != nil {
		return err
	}

	// 创建默认发货地点
	return prefs.EnsureDefaultLocation(locationID, locationName)
}

// convertLegacyOptionToZone 将最早版本的 ShippingOption 转换为 ShippingZone
func convertLegacyOptionToZone(option *ShippingOption) *ShippingZone {
	if option == nil {
		return nil
	}

	// 生成唯一 ID（使用简单的格式）
	zoneID := generateZoneID(option.Name)

	// 将 Services 转换为 Rates
	rates := make([]*ShippingRate, 0, len(option.Services))
	for i, service := range option.Services {
		rateID := generateRateID(service.Name, i)
		rate := &ShippingRate{
			ID:                rateID,
			Name:              service.Name,
			Price:             service.FirstFreight, // 使用首重费用作为固定价格
			Currency:          option.Currency,
			EstimatedDelivery: service.EstimatedDelivery,
		}
		rates = append(rates, rate)
	}

	// 如果没有服务，创建一个默认费率
	if len(rates) == 0 {
		rates = append(rates, &ShippingRate{
			ID:       "default_rate",
			Name:     option.Name,
			Price:    "0",
			Currency: option.Currency,
		})
	}

	// 处理满额免邮（应用到第一个费率）
	if option.FreeShippingThreshold != nil && len(rates) > 0 {
		rates[0].FreeShippingThreshold = option.FreeShippingThreshold
	}

	return &ShippingZone{
		ID:      zoneID,
		Name:    option.Name,
		Regions: option.Regions,
		Rates:   rates,
	}
}

// generateZoneID 生成区域 ID
func generateZoneID(name string) string {
	// 简单实现：使用名称的小写形式加后缀
	return strings.ToLower(strings.ReplaceAll(name, " ", "_")) + "_zone"
}

// generateRateID 生成费率 ID
func generateRateID(name string, index int) string {
	base := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	return fmt.Sprintf("%s_rate_%d", base, index)
}

// GetMigrationDefaultProfileID 获取迁移后的默认档案 ID（用于更新现有商品）
func (prefs *UserPreferences) GetMigrationDefaultProfileID() string {
	profile, err := prefs.GetDefaultShippingProfile()
	if err != nil || profile == nil {
		return ""
	}
	return profile.ProfileID
}
