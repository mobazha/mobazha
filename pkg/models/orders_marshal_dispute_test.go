package models

import (
	"testing"

	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestMergeDisputeOpenAPIFields_fillsEvidenceFromDB(t *testing.T) {
	payload := map[string]interface{}{
		"disputeOpen": map[string]interface{}{
			"reason": "Item not as described",
		},
	}
	o := &Order{
		DisputeEvidenceHashes: StringSlice{"QmEvidenceHash1", "QmEvidenceHash2"},
	}
	mergeDisputeOpenAPIFields(payload, o)

	dom := payload["disputeOpen"].(map[string]interface{})
	hashes, ok := dom["evidenceHashes"].([]interface{})
	if !ok || len(hashes) != 2 {
		t.Fatalf("expected 2 evidence hashes, got %#v", dom["evidenceHashes"])
	}
	if hashes[0] != "QmEvidenceHash1" {
		t.Fatalf("unexpected hash[0]: %v", hashes[0])
	}
}

func TestMergeDisputeOpenAPIFields_preservesExistingHashes(t *testing.T) {
	payload := map[string]interface{}{
		"disputeOpen": map[string]interface{}{
			"evidenceHashes": []interface{}{"QmExisting"},
		},
	}
	o := &Order{
		DisputeEvidenceHashes: StringSlice{"QmFromDB"},
	}
	mergeDisputeOpenAPIFields(payload, o)

	dom := payload["disputeOpen"].(map[string]interface{})
	hashes := dom["evidenceHashes"].([]interface{})
	if len(hashes) != 1 || hashes[0] != "QmExisting" {
		t.Fatalf("expected preserved serialized hashes, got %#v", hashes)
	}
}

func TestMergeDisputeOpenAPIFields_fillsFromProtobufWhenDBEmpty(t *testing.T) {
	payload := map[string]interface{}{
		"disputeOpen": map[string]interface{}{"reason": "test"},
	}
	disputeOpenAny := &anypb.Any{}
	if err := disputeOpenAny.MarshalFrom(&pb.DisputeOpen{
		Reason:         "test",
		EvidenceHashes: []string{"QmFromProto"},
	}); err != nil {
		t.Fatal(err)
	}
	o := &Order{}
	if err := o.PutMessage(&npb.OrderMessage{
		Signature:   []byte("sig"),
		MessageType: npb.OrderMessage_DISPUTE_OPEN,
		Message:     disputeOpenAny,
	}); err != nil {
		t.Fatal(err)
	}
	mergeDisputeOpenAPIFields(payload, o)

	dom := payload["disputeOpen"].(map[string]interface{})
	hashes, ok := dom["evidenceHashes"].([]interface{})
	if !ok || len(hashes) != 1 || hashes[0] != "QmFromProto" {
		t.Fatalf("expected protobuf evidence hash, got %#v", dom["evidenceHashes"])
	}
}

func TestMergeDisputeOpenAPIFields_createsDisputeOpenWhenMissing(t *testing.T) {
	payload := map[string]interface{}{}
	o := &Order{
		DisputeEvidenceHashes: StringSlice{"QmOnlyDB"},
	}
	mergeDisputeOpenAPIFields(payload, o)
	dom, ok := payload["disputeOpen"].(map[string]interface{})
	if !ok {
		t.Fatal("expected disputeOpen map to be created")
	}
	hashes := dom["evidenceHashes"].([]interface{})
	if len(hashes) != 1 || hashes[0] != "QmOnlyDB" {
		t.Fatalf("unexpected hashes: %#v", hashes)
	}
}
