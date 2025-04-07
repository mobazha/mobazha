package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestOpenBazaarNode_Profile(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}
	defer node.repo.DestroyRepo()

	name := "Ron Swanson"

	err = node.SetProfile(&models.Profile{
		Name:            name,
		EscrowPublicKey: strings.Repeat("s", 66),
	}, nil)
	if err != nil {
		t.Error(err)
	}

	pro, err := node.GetMyProfile()
	if err != nil {
		t.Error(err)
	}

	if pro.Name != name {
		t.Errorf("Returned incorrect profile. Expected name %s go %s", name, pro.Name)
	}
}

func TestOpenBazaarNode_GetProfile(t *testing.T) {
	mocknet, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}
	defer mocknet.TearDown()

	var (
		name      = "Ron Swanson"
		snfServer = "12D3KooWBiB44mFtWYTGstSiDKMsA89ztk9bPTjh57uW74U4XDpa"
	)
	done := make(chan struct{})
	mocknet.Nodes()[0].storeAndForwardServers = []string{snfServer}
	err = mocknet.Nodes()[0].SetProfile(&models.Profile{
		Name:            name,
		EscrowPublicKey: strings.Repeat("s", 66),
	}, done)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	pro, err := mocknet.Nodes()[1].GetProfile(context.Background(), mocknet.Nodes()[0].Identity(), false)
	if err != nil {
		t.Fatal(err)
	}

	if pro.Name != name {
		t.Errorf("Returned profile with incorrect name. Expected %s, got %s", name, pro.Name)
	}

	// Change name
	name2 := "Peter Griffin"
	done = make(chan struct{})
	err = mocknet.Nodes()[0].SetProfile(&models.Profile{
		Name:            name2,
		EscrowPublicKey: strings.Repeat("s", 66),
	}, done)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	// Test fetching from cache
	pro, err = mocknet.Nodes()[1].GetProfile(context.Background(), mocknet.Nodes()[0].Identity(), true)
	if err != nil {
		t.Fatal(err)
	}

	if pro.Name != name {
		t.Errorf("Returned profile with incorrect name. Expected %s, got %s", name, pro.Name)
	}

	var record models.StoreAndForwardServers
	err = mocknet.Nodes()[1].repo.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("peer_id=?", mocknet.Nodes()[0].Identity().String()).First(&record).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	peerIDs, err := record.Servers()
	if err != nil {
		t.Fatal(err)
	}
	if len(peerIDs) != 1 {
		t.Errorf("Incorrect peerIDs returned. Expected 1 got %d", len(peerIDs))
	}

	if peerIDs[0].String() != snfServer {
		t.Errorf("Incorrect snf server. Expected %s, got %s", snfServer, peerIDs[0])
	}
}

func Test_updateProfileStats(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	var (
		name    = "Ron Paul"
		profile = &models.Profile{Name: name}
	)
	err = node.repo.DB().Update(func(tx database.Tx) error {
		if err := tx.SetFollowers(models.Followers{"QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub"}); err != nil {
			return err
		}

		if err := tx.SetFollowing(models.Following{"QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub"}); err != nil {
			return err
		}

		return node.updateProfileStats(tx, profile)
	})
	if err != nil {
		t.Fatal(err)
	}

	if profile.Name != name {
		t.Errorf("Incorrect profile name. Expected %s got %s", name, profile.Name)
	}

	if profile.Stats == nil {
		t.Error("Profile stats is nil")
	}

	if profile.Stats.FollowerCount != 1 {
		t.Errorf("Incorrect follower count. Expected 1 got %d", profile.Stats.FollowerCount)
	}

	if profile.Stats.FollowingCount != 1 {
		t.Errorf("Incorrect following count. Expected 1 got %d", profile.Stats.FollowingCount)
	}
}

func Test_updateAndSaveProfile(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	var (
		name    = "Ron Paul"
		profile = &models.Profile{Name: name}
	)
	if err := node.SetProfile(profile, nil); err != nil {
		t.Fatal(err)
	}
	err = node.repo.DB().Update(func(tx database.Tx) error {
		if err := tx.SetFollowers(models.Followers{"QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub"}); err != nil {
			return err
		}

		if err := tx.SetFollowing(models.Following{"QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub"}); err != nil {
			return err
		}

		return node.updateAndSaveProfile(tx)
	})
	if err != nil {
		t.Fatal(err)
	}

	ret, err := node.GetMyProfile()
	if err != nil {
		t.Fatal(err)
	}

	if ret.Name != name {
		t.Errorf("Incorrect profile name. Expected %s got %s", name, ret.Name)
	}

	if ret.Stats == nil {
		t.Error("Profile stats is nil")
	}

	if ret.Stats.FollowerCount != 1 {
		t.Errorf("Incorrect follower count. Expected 1 got %d", ret.Stats.FollowerCount)
	}

	if ret.Stats.FollowingCount != 1 {
		t.Errorf("Incorrect following count. Expected 1 got %d", ret.Stats.FollowingCount)
	}
}

func Test_updateSNFServers(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	var (
		name    = "Ron Paul"
		profile = &models.Profile{Name: name}
	)

	if err := node.SetProfile(profile, nil); err != nil {
		t.Fatal(err)
	}

	server := "adsf"
	node.storeAndForwardServers = []string{server}

	err = node.updateSNFServers()
	if err != nil {
		t.Fatal(err)
	}

	pro, err := node.GetMyProfile()
	if err != nil {
		t.Fatal(err)
	}

	if pro.StoreAndForwardServers[0] != server {
		t.Errorf("Expected server %s, got %s", server, pro.StoreAndForwardServers[0])
	}
}

func TestOpenBazaarNode_validateProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile *models.Profile
		valid   bool
	}{
		{
			name: "Valid name",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("s", 66),
			},
			valid: true,
		},
		{
			name: "Name zero len",
			profile: &models.Profile{
				Name:            "",
				EscrowPublicKey: strings.Repeat("s", 66),
			},
			valid: false,
		},
		{
			name: "Name > max len",
			profile: &models.Profile{
				Name:            strings.Repeat("r", WordMaxCharacters+1),
				EscrowPublicKey: strings.Repeat("s", 66),
			},
			valid: false,
		},
		{
			name: "Name at max len",
			profile: &models.Profile{
				Name:            strings.Repeat("r", WordMaxCharacters),
				EscrowPublicKey: strings.Repeat("s", 66),
			},
			valid: true,
		},
		{
			name: "Location at max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				Location:        strings.Repeat("r", WordMaxCharacters),
				EscrowPublicKey: strings.Repeat("s", 66),
			},
			valid: true,
		},
		{
			name: "Location > max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				Location:        strings.Repeat("r", WordMaxCharacters+1),
				EscrowPublicKey: strings.Repeat("s", 66),
			},
			valid: false,
		},
		{
			name: "About at max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				About:           strings.Repeat("r", AboutMaxCharacters),
				EscrowPublicKey: strings.Repeat("s", 66),
			},
			valid: true,
		},
		{
			name: "About > max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				About:           strings.Repeat("r", AboutMaxCharacters+1),
				EscrowPublicKey: strings.Repeat("s", 66),
			},
			valid: false,
		},
		{
			name: "Short description at max len",
			profile: &models.Profile{
				Name:             "Ron Swanson",
				ShortDescription: strings.Repeat("r", models.ShortDescriptionLength),
				EscrowPublicKey:  strings.Repeat("s", 66),
			},
			valid: true,
		},
		{
			name: "Short description > max len",
			profile: &models.Profile{
				Name:             "Ron Swanson",
				ShortDescription: strings.Repeat("r", models.ShortDescriptionLength+1),
				EscrowPublicKey:  strings.Repeat("s", 66),
			},
			valid: false,
		},
		{
			name: "Public key correct len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
			},
			valid: true,
		},
		{
			name: "Public key != len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 67),
			},
			valid: false,
		},
		{
			name: "Contact info:website correct len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ContactInfo: &models.ProfileContactInfo{
					Website: strings.Repeat("s", URLMaxCharacters),
				},
			},
			valid: true,
		},
		{
			name: "Contact info:website > max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ContactInfo: &models.ProfileContactInfo{
					Website: strings.Repeat("s", URLMaxCharacters+1),
				},
			},
			valid: false,
		},
		{
			name: "Contact info:email correct len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ContactInfo: &models.ProfileContactInfo{
					Email: strings.Repeat("s", SentenceMaxCharacters),
				},
			},
			valid: true,
		},
		{
			name: "Contact info:email > max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ContactInfo: &models.ProfileContactInfo{
					Email: strings.Repeat("s", SentenceMaxCharacters+1),
				},
			},
			valid: false,
		},
		{
			name: "Contact info:phone number correct len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ContactInfo: &models.ProfileContactInfo{
					PhoneNumber: strings.Repeat("s", WordMaxCharacters),
				},
			},
			valid: true,
		},
		{
			name: "Contact info:phone number > max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ContactInfo: &models.ProfileContactInfo{
					PhoneNumber: strings.Repeat("s", WordMaxCharacters+1),
				},
			},
			valid: false,
		},
		{
			name: "Contact info:social list correct len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ContactInfo: &models.ProfileContactInfo{
					Social: make([]models.SocialAccount, MaxListItems),
				},
			},
			valid: true,
		},
		{
			name: "Contact info:social list > max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ContactInfo: &models.ProfileContactInfo{
					Social: make([]models.SocialAccount, MaxListItems+1),
				},
			},
			valid: false,
		},
		{
			name: "Contact info:social:username correct len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ContactInfo: &models.ProfileContactInfo{
					Social: []models.SocialAccount{
						{
							Username: strings.Repeat("s", WordMaxCharacters),
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "Contact info:social:username > max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ContactInfo: &models.ProfileContactInfo{
					Social: []models.SocialAccount{
						{
							Username: strings.Repeat("s", WordMaxCharacters+1),
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "Contact info:social:type correct len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ContactInfo: &models.ProfileContactInfo{
					Social: []models.SocialAccount{
						{
							Type: strings.Repeat("s", WordMaxCharacters),
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "Contact info:social:type > max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ContactInfo: &models.ProfileContactInfo{
					Social: []models.SocialAccount{
						{
							Type: strings.Repeat("s", WordMaxCharacters+1),
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "Contact info:social:proof correct len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ContactInfo: &models.ProfileContactInfo{
					Social: []models.SocialAccount{
						{
							Proof: strings.Repeat("s", URLMaxCharacters),
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "Contact info:social:proof > max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ContactInfo: &models.ProfileContactInfo{
					Social: []models.SocialAccount{
						{
							Proof: strings.Repeat("s", URLMaxCharacters+1),
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "Mod info:description correct len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ModeratorInfo: &models.ModeratorInfo{
					Description: strings.Repeat("s", AboutMaxCharacters),
					Fee: models.ModeratorFee{
						FeeType: models.PercentageFee,
					},
				},
			},
			valid: true,
		},
		{
			name: "Mod info:description > max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ModeratorInfo: &models.ModeratorInfo{
					Description: strings.Repeat("s", AboutMaxCharacters+1),
				},
			},
			valid: false,
		},
		{
			name: "Mod info:terms and conditions correct len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ModeratorInfo: &models.ModeratorInfo{
					TermsAndConditions: strings.Repeat("s", PolicyMaxCharacters),
					Fee: models.ModeratorFee{
						FeeType: models.PercentageFee,
					},
				},
			},
			valid: true,
		},
		{
			name: "Mod info:terms and conditions > max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ModeratorInfo: &models.ModeratorInfo{
					TermsAndConditions: strings.Repeat("s", PolicyMaxCharacters+1),
				},
			},
			valid: false,
		},
		{
			name: "Mod info:language list correct len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ModeratorInfo: &models.ModeratorInfo{
					Languages: make([]string, MaxListItems),
					Fee: models.ModeratorFee{
						FeeType: models.PercentageFee,
					},
				},
			},
			valid: true,
		},
		{
			name: "Mod info:languages list > max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ModeratorInfo: &models.ModeratorInfo{
					Languages: make([]string, MaxListItems+1),
				},
			},
			valid: false,
		},
		{
			name: "Mod info:language correct len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ModeratorInfo: &models.ModeratorInfo{
					Languages: []string{
						strings.Repeat("s", WordMaxCharacters),
					},
					Fee: models.ModeratorFee{
						FeeType: models.PercentageFee,
					},
				},
			},
			valid: true,
		},
		{
			name: "Mod info:languages > max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ModeratorInfo: &models.ModeratorInfo{
					Languages: []string{
						strings.Repeat("s", WordMaxCharacters+1),
					},
				},
			},
			valid: false,
		},
		{
			name: "Mod info:fixed fee:currency code correct len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ModeratorInfo: &models.ModeratorInfo{
					Fee: models.ModeratorFee{
						FixedFee: &models.CurrencyValue{
							Currency: &models.Currency{
								Name:         strings.Repeat("s", WordMaxCharacters),
								Code:         models.CurrencyCode(strings.Repeat("s", WordMaxCharacters)),
								CurrencyType: models.CurrencyType(strings.Repeat("s", WordMaxCharacters)),
							},
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "Mod info:fixed fee: currency code > max len",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				ModeratorInfo: &models.ModeratorInfo{
					Fee: models.ModeratorFee{
						FixedFee: &models.CurrencyValue{
							Currency: &models.Currency{
								Name:         strings.Repeat("s", WordMaxCharacters+1),
								Code:         models.CurrencyCode(strings.Repeat("s", WordMaxCharacters+1)),
								CurrencyType: models.CurrencyType(strings.Repeat("s", WordMaxCharacters+1)),
							},
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "Valid avatar hashes",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				AvatarHashes: models.ImageHashes{
					Large:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
					Medium:   "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
					Original: "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
					Small:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
					Tiny:     "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
				},
			},
			valid: true,
		},
		{
			name: "Invalid avatar hashes",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				AvatarHashes: models.ImageHashes{
					Large:    "xxx",
					Medium:   "xxx",
					Original: "xxx",
					Small:    "xxx",
					Tiny:     "xxx",
				},
			},
			valid: false,
		},
		{
			name: "Valid header hashes",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				HeaderHashes: models.ImageHashes{
					Large:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
					Medium:   "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
					Original: "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
					Small:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
					Tiny:     "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
				},
			},
			valid: true,
		},
		{
			name: "Invalid header hashes",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				HeaderHashes: models.ImageHashes{
					Large:    "xxx",
					Medium:   "xxx",
					Original: "xxx",
					Small:    "xxx",
					Tiny:     "xxx",
				},
			},
			valid: false,
		},
		{
			name: "Average rating correct range",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				Stats: &models.ProfileStats{
					AverageRating: 5,
				},
			},
			valid: true,
		},
		{
			name: "Average rating incorrect range",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				Stats: &models.ProfileStats{
					AverageRating: 6,
				},
			},
			valid: false,
		},
		{
			name: "Valid offline inboxes",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				StoreAndForwardServers: []string{
					"12D3KooWBiB44mFtWYTGstSiDKMsA89ztk9bPTjh57uW74U4XDpa",
				},
			},
			valid: true,
		},
		{
			name: "Invalid offline inbox",
			profile: &models.Profile{
				Name:            "Ron Swanson",
				EscrowPublicKey: strings.Repeat("r", 66),
				StoreAndForwardServers: []string{
					"asdfaf",
				},
			},
			valid: false,
		},
	}

	for _, test := range tests {
		err := validateProfile(test.profile)
		if test.valid && err != nil {
			t.Errorf("Test %s: returned unexpected error: %s", test.name, err)
		} else if !test.valid && err == nil {
			t.Errorf("Test %s: did not return error", test.name)
		}
	}
}
