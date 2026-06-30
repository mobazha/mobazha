//go:build !private_distribution

package core

import (
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
	"gorm.io/gorm"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

func TestClassifyRoomFromState_DirectMetadataOverridesMemberCount(t *testing.T) {
	stateMap := mautrix.RoomStateMap{
		roomMetadataEventType: map[string]*event.Event{
			"": {
				Content: event.Content{
					Raw: map[string]interface{}{"type": "direct"},
				},
			},
		},
	}

	if got := classifyRoomFromState(stateMap, 1); got != "direct" {
		t.Fatalf("classifyRoomFromState() = %q, want %q", got, "direct")
	}
}

func TestClassifyRoomFromState_TwoMembersDefaultsToDirect(t *testing.T) {
	if got := classifyRoomFromState(mautrix.RoomStateMap{}, 2); got != "direct" {
		t.Fatalf("classifyRoomFromState() = %q, want %q", got, "direct")
	}
}

func TestCustomStateEventTypes_UseStateClass(t *testing.T) {
	if peerIDEventType.Class != event.StateEventType {
		t.Fatalf("peerIDEventType class = %v, want %v", peerIDEventType.Class, event.StateEventType)
	}
	if roomMetadataEventType.Class != event.StateEventType {
		t.Fatalf(
			"roomMetadataEventType class = %v, want %v",
			roomMetadataEventType.Class,
			event.StateEventType,
		)
	}
}

func TestLookupCanonicalPeerIDsByMatrixUserID_ResolvesLocalMobazhaMembers(t *testing.T) {
	service := newTestService()
	service.config.DB = newMatrixCredentialsTestDB(t, []models.MatrixCredentials{
		{
			PeerID:       "12D3KooWAlice",
			MatrixUserID: "@peer_12d3koowalice:matrix.local",
			ServerName:   "matrix.local",
			Registered:   true,
		},
	})

	got := service.lookupCanonicalPeerIDsByMatrixUserID([]string{
		"@peer_12d3koowalice:matrix.local",
		"@external:other.example",
	})

	if got["@peer_12d3koowalice:matrix.local"] != "12D3KooWAlice" {
		t.Fatalf("expected canonical peerID for local member, got %#v", got)
	}
	if _, ok := got["@external:other.example"]; ok {
		t.Fatalf("did not expect external Matrix member to resolve to a peerID: %#v", got)
	}
}

func TestFillMissingMemberPeerIDs_PreservesExplicitStateAndFillsMissing(t *testing.T) {
	service := newTestService()
	service.config.DB = newMatrixCredentialsTestDB(t, []models.MatrixCredentials{
		{
			PeerID:       "12D3KooWBob",
			MatrixUserID: "@peer_12d3koowbob:matrix.local",
			ServerName:   "matrix.local",
			Registered:   true,
		},
	})

	members := []contracts.MatrixMember{
		{
			UserID:      "@peer_12d3koowalice:matrix.local",
			DisplayName: "Alice",
			PeerID:      "12D3KooWAlice",
			Membership:  "join",
		},
		{
			UserID:      "@peer_12d3koowbob:matrix.local",
			DisplayName: "Bob",
			Membership:  "join",
		},
		{
			UserID:      "@external:other.example",
			DisplayName: "Eve",
			Membership:  "join",
		},
	}

	service.fillMissingMemberPeerIDs(members)

	if members[0].PeerID != "12D3KooWAlice" {
		t.Fatalf("expected explicit state peerID to be preserved, got %q", members[0].PeerID)
	}
	if members[1].PeerID != "12D3KooWBob" {
		t.Fatalf("expected missing peerID to be resolved from credentials, got %q", members[1].PeerID)
	}
	if members[2].PeerID != "" {
		t.Fatalf("expected external Matrix member to remain without peerID, got %q", members[2].PeerID)
	}
}

func TestSelfPeerIDAssignments_ReturnsCurrentMemberOnly(t *testing.T) {
	service := newTestService()
	service.config.PeerID = mustPeerID(t, testVendorPeerID)

	got := service.selfPeerIDAssignments()

	if got[service.matrixUserID.String()] != testVendorPeerID {
		t.Fatalf("expected self peerID assignment, got %#v", got)
	}
	if len(got) != 1 {
		t.Fatalf("expected only self assignment, got %#v", got)
	}
}

func TestSelfPeerIDAssignments_ReturnsNilWithoutConfiguredPeerID(t *testing.T) {
	service := newTestService()
	got := service.selfPeerIDAssignments()

	if got != nil {
		t.Fatalf("expected nil assignments without configured peerID, got %#v", got)
	}
}

func TestFillDirectRoomMemberPeerIDFromMetadata_AssignsTargetPeerID(t *testing.T) {
	service := newTestService()
	members := []contracts.MatrixMember{
		{
			UserID:      "@peer_me:matrix.local",
			DisplayName: "Me",
			PeerID:      "12D3KooWMe",
			Membership:  "join",
		},
		{
			UserID:      "@peer_alice:matrix.local",
			DisplayName: "Alice",
			Membership:  "join",
		},
	}

	service.fillDirectRoomMemberPeerIDFromMetadata(members, map[string]string{
		directRoomTargetPeerIDMetaKey: "12D3KooWAlice",
	})

	if members[1].PeerID != "12D3KooWAlice" {
		t.Fatalf("expected metadata peerID assignment, got %#v", members)
	}
}

func TestFillDirectRoomMemberPeerIDFromMetadata_DoesNotOverrideExistingPeerID(t *testing.T) {
	service := newTestService()
	members := []contracts.MatrixMember{
		{
			UserID:      "@peer_alice:matrix.local",
			DisplayName: "Alice",
			PeerID:      "12D3KooWAliceFromState",
			Membership:  "join",
		},
	}

	service.fillDirectRoomMemberPeerIDFromMetadata(members, map[string]string{
		directRoomTargetPeerIDMetaKey: "12D3KooWAliceFromMetadata",
	})

	if members[0].PeerID != "12D3KooWAliceFromState" {
		t.Fatalf("expected state peerID to win, got %#v", members)
	}
}

func TestResolveDirectRoomTarget_FromTargetPeerID(t *testing.T) {
	service := newTestService()
	targetPeerID := "12D3KooWBkkycUCusJiLHXogEfiHghmMy3kDgtSovn58zy9uwikB"

	targetUserID, targetPeer, err := service.resolveDirectRoomTarget(contracts.MatrixDirectRoomTarget{
		TargetPeerID: targetPeerID,
	})
	if err != nil {
		t.Fatalf("resolveDirectRoomTarget() error = %v", err)
	}
	if targetPeer != targetPeerID {
		t.Fatalf("targetPeer = %q, want %q", targetPeer, targetPeerID)
	}
	expectedUserID := "@peer_12d3koowbkkycucusjilhxogefihghmmy3kdgtsovn58zy9uwikb:matrix.test.local"
	if string(targetUserID) != expectedUserID {
		t.Fatalf("targetUserID = %q, want %q", targetUserID, expectedUserID)
	}
}

func TestResolveDirectRoomTarget_NormalizesPeerIDFromCredentials(t *testing.T) {
	service := newTestService()

	canonicalPeerID := "12D3KooWBkkycUCusJiLHXogEfiHghmMy3kDgtSovn58zy9uwikB"
	matrixUserID := "@peer_12d3koowbkkycucusjilhxogefihghmmy3kdgtsovn58zy9uwikb:matrix.test.local"
	service.config.DB = newMatrixCredentialsTestDB(t, []models.MatrixCredentials{
		{
			PeerID:       canonicalPeerID,
			MatrixUserID: matrixUserID,
			ServerName:   "matrix.test.local",
			Registered:   true,
		},
	})

	targetUserID, targetPeer, err := service.resolveDirectRoomTarget(contracts.MatrixDirectRoomTarget{
		TargetPeerID: "12d3koowbkkycucusjilhxogefihghmmy3kdgtsovn58zy9uwikb",
	})
	if err != nil {
		t.Fatalf("resolveDirectRoomTarget() error = %v", err)
	}
	if string(targetUserID) != matrixUserID {
		t.Fatalf("targetUserID = %q, want %q", targetUserID, matrixUserID)
	}
	if targetPeer != canonicalPeerID {
		t.Fatalf("targetPeer = %q, want %q", targetPeer, canonicalPeerID)
	}
}

func TestResolveDirectRoomTarget_FromTargetUserID(t *testing.T) {
	service := newTestService()

	targetUserID, targetPeer, err := service.resolveDirectRoomTarget(contracts.MatrixDirectRoomTarget{
		TargetUserID: "@alice:matrix.org",
	})
	if err != nil {
		t.Fatalf("resolveDirectRoomTarget() error = %v", err)
	}
	if string(targetUserID) != "@alice:matrix.org" {
		t.Fatalf("targetUserID = %q, want %q", targetUserID, "@alice:matrix.org")
	}
	if targetPeer != "" {
		t.Fatalf("targetPeer = %q, want empty", targetPeer)
	}
}

func TestResolveDirectRoomTarget_RejectsMissingAndAmbiguousTarget(t *testing.T) {
	service := newTestService()

	if _, _, err := service.resolveDirectRoomTarget(contracts.MatrixDirectRoomTarget{}); err == nil {
		t.Fatalf("expected error for missing target")
	}
	if _, _, err := service.resolveDirectRoomTarget(contracts.MatrixDirectRoomTarget{
		TargetUserID: "@alice:matrix.org",
		TargetPeerID: "12D3KooWBkkycUCusJiLHXogEfiHghmMy3kDgtSovn58zy9uwikB",
	}); err == nil {
		t.Fatalf("expected error when both targetUserID and targetPeerID are set")
	}
}

func TestResolveDirectRoomTarget_RejectsSelfTarget(t *testing.T) {
	service := newTestService()

	if _, _, err := service.resolveDirectRoomTarget(contracts.MatrixDirectRoomTarget{
		TargetUserID: service.matrixUserID.String(),
	}); err == nil {
		t.Fatalf("expected error when target user is self")
	}
}

func TestFillDirectRoomMemberPeerIDFromMetadata_DoesNotAssignWhenMetadataPeerIDIsSelf(t *testing.T) {
	service := newTestService()
	service.config.PeerID = mustPeerID(t, testVendorPeerID)

	members := []contracts.MatrixMember{
		{
			UserID:      service.matrixUserID.String(),
			DisplayName: "Me",
			PeerID:      testVendorPeerID,
			Membership:  "join",
		},
		{
			UserID:      "@peer_other:matrix.local",
			DisplayName: "Other",
			Membership:  "join",
		},
	}

	service.fillDirectRoomMemberPeerIDFromMetadata(members, map[string]string{
		"type":                        "direct",
		directRoomTargetPeerIDMetaKey: testVendorPeerID,
	})

	if members[1].PeerID != "" {
		t.Fatalf("expected counterparty peerID to remain empty, got %#v", members)
	}
}

type matrixCredentialsTestDB struct {
	db *gorm.DB
}

func newMatrixCredentialsTestDB(t *testing.T, records []models.MatrixCredentials) database.Database {
	t.Helper()

	gdb, err := gorm.Open(sqlitedialect.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := gdb.AutoMigrate(&models.MatrixCredentials{}); err != nil {
		t.Fatalf("migrate matrix credentials: %v", err)
	}
	for _, record := range records {
		if err := gdb.Create(&record).Error; err != nil {
			t.Fatalf("seed matrix credentials: %v", err)
		}
	}
	return matrixCredentialsTestDB{db: gdb}
}

func (db matrixCredentialsTestDB) View(fn func(tx database.Tx) error) error {
	return fn(matrixCredentialsTestTx{db: db.db})
}

func (db matrixCredentialsTestDB) Update(fn func(tx database.Tx) error) error {
	return fn(matrixCredentialsTestTx{db: db.db})
}

func (db matrixCredentialsTestDB) ComputePublicDataHash() (cid.Cid, error) {
	return cid.Cid{}, nil
}

func (db matrixCredentialsTestDB) Close() error {
	return nil
}

type matrixCredentialsTestTx struct {
	db *gorm.DB
}

func (tx matrixCredentialsTestTx) Commit() error            { return nil }
func (tx matrixCredentialsTestTx) Rollback() error          { return nil }
func (tx matrixCredentialsTestTx) Read() *gorm.DB           { return tx.db }
func (tx matrixCredentialsTestTx) Save(interface{}) error   { return nil }
func (tx matrixCredentialsTestTx) Create(interface{}) error { return nil }
func (tx matrixCredentialsTestTx) Update(string, interface{}, map[string]interface{}, interface{}) error {
	return nil
}
func (tx matrixCredentialsTestTx) UpdateColumns(map[string]interface{}, map[string]interface{}, interface{}) (int64, error) {
	return 0, nil
}
func (tx matrixCredentialsTestTx) Delete(string, interface{}, map[string]interface{}, interface{}) error {
	return nil
}
func (tx matrixCredentialsTestTx) DeleteAll(interface{}) error { return nil }
func (tx matrixCredentialsTestTx) Migrate(interface{}) error   { return nil }
func (tx matrixCredentialsTestTx) RegisterCommitHook(func())   {}
func (tx matrixCredentialsTestTx) GetProfile() (*models.Profile, error) {
	return nil, nil
}
func (tx matrixCredentialsTestTx) SetProfile(*models.Profile) error { return nil }
func (tx matrixCredentialsTestTx) GetFollowers() (models.Followers, error) {
	return nil, nil
}
func (tx matrixCredentialsTestTx) SetFollowers(models.Followers) error { return nil }
func (tx matrixCredentialsTestTx) GetFollowing() (models.Following, error) {
	return nil, nil
}
func (tx matrixCredentialsTestTx) SetFollowing(models.Following) error { return nil }
func (tx matrixCredentialsTestTx) GetListing(string) (*pb.SignedListing, error) {
	return nil, nil
}
func (tx matrixCredentialsTestTx) SetListing(*pb.SignedListing) error { return nil }
func (tx matrixCredentialsTestTx) GetEncryptedListing(string) ([]byte, error) {
	return nil, nil
}
func (tx matrixCredentialsTestTx) SetEncryptedListing(string, []byte) error { return nil }
func (tx matrixCredentialsTestTx) DeleteListing(string) error               { return nil }
func (tx matrixCredentialsTestTx) GetListingIndex() (models.ListingIndex, error) {
	return nil, nil
}
func (tx matrixCredentialsTestTx) SetListingIndex(models.ListingIndex) error { return nil }
func (tx matrixCredentialsTestTx) GetRatingIndex() (models.RatingIndex, error) {
	return nil, nil
}
func (tx matrixCredentialsTestTx) SetRatingIndex(models.RatingIndex) error { return nil }
func (tx matrixCredentialsTestTx) SetRating(*pb.Rating) error              { return nil }
func (tx matrixCredentialsTestTx) GetPostIndex() ([]models.PostData, error) {
	return nil, nil
}
func (tx matrixCredentialsTestTx) SetPostIndex([]models.PostData) error { return nil }
func (tx matrixCredentialsTestTx) AddPost(*postsPb.SignedPost) error    { return nil }
func (tx matrixCredentialsTestTx) DeletePost(string) error              { return nil }
func (tx matrixCredentialsTestTx) PostExist(string) bool                { return false }
func (tx matrixCredentialsTestTx) GetPost(string) (*postsPb.SignedPost, error) {
	return nil, nil
}
func (tx matrixCredentialsTestTx) SetImage(models.Image) error { return nil }
func (tx matrixCredentialsTestTx) GetImageByName(models.ImageSize, string) ([]byte, error) {
	return nil, nil
}
func (tx matrixCredentialsTestTx) GetMediaByCID(string) ([]byte, string, error) {
	return nil, "", nil
}
func (tx matrixCredentialsTestTx) IndexMediaCID(string, string, string, string, string) error {
	return nil
}
func (tx matrixCredentialsTestTx) SetUploadedFile(models.UploadedFile) error { return nil }
func (tx matrixCredentialsTestTx) SetIntroVideo(models.IntroVideo) error     { return nil }

var _ database.Database = matrixCredentialsTestDB{}
var _ database.Tx = matrixCredentialsTestTx{}
var _ database.Database = matrixCredentialsTestDB{}
