package api

import (
	"context"
	"errors"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

// fakeCollectionService is a minimal CollectionService stub for
// filterListingsByCollectionsWithService tests. Only IsProductInCollections
// is meaningfully implemented; the rest return zero values so we satisfy
// the interface without pulling in the full CollectionAppService.
type fakeCollectionService struct {
	contracts.CollectionService // embed to satisfy interface; methods below override as needed

	// membership[slug] = collection IDs that include the slug
	membership map[string][]string
	err        error
	calls      int
}

func (f *fakeCollectionService) IsProductInCollections(_ context.Context, collectionIDs []string, slug string) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	inCols, ok := f.membership[slug]
	if !ok {
		return false, nil
	}
	set := make(map[string]struct{}, len(inCols))
	for _, c := range inCols {
		set[c] = struct{}{}
	}
	for _, want := range collectionIDs {
		if _, ok := set[want]; ok {
			return true, nil
		}
	}
	return false, nil
}

func TestFilterListingsByCollectionsWithService(t *testing.T) {
	index := models.ListingIndex{
		models.ListingMetadata{Slug: "alpha"},
		models.ListingMetadata{Slug: "beta"},
		models.ListingMetadata{Slug: "gamma"},
	}

	t.Run("nil service returns input unchanged", func(t *testing.T) {
		out, err := filterListingsByCollectionsWithService(context.Background(), nil, index, []string{"col-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) != len(index) {
			t.Fatalf("expected %d listings, got %d", len(index), len(out))
		}
	})

	t.Run("empty collectionIDs returns input unchanged", func(t *testing.T) {
		cs := &fakeCollectionService{}
		out, err := filterListingsByCollectionsWithService(context.Background(), cs, index, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) != len(index) {
			t.Fatalf("expected %d listings, got %d", len(index), len(out))
		}
		if cs.calls != 0 {
			t.Fatalf("expected no service calls when filter is empty, got %d", cs.calls)
		}
	})

	t.Run("keeps only listings in given collections", func(t *testing.T) {
		cs := &fakeCollectionService{
			membership: map[string][]string{
				"alpha": {"col-featured"},
				"beta":  {"col-other"},
				"gamma": {"col-featured", "col-extra"},
			},
		}
		out, err := filterListingsByCollectionsWithService(context.Background(), cs, index, []string{"col-featured"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) != 2 {
			t.Fatalf("expected 2 listings (alpha, gamma), got %d: %+v", len(out), out)
		}
		gotSlugs := map[string]bool{}
		for _, l := range out {
			gotSlugs[l.Slug] = true
		}
		if !gotSlugs["alpha"] || !gotSlugs["gamma"] {
			t.Fatalf("expected alpha and gamma kept, got %+v", gotSlugs)
		}
		if gotSlugs["beta"] {
			t.Fatalf("beta should have been filtered out, got %+v", gotSlugs)
		}
		if cs.calls != 3 {
			t.Fatalf("expected one call per listing (3), got %d", cs.calls)
		}
	})

	t.Run("empty result when nothing matches", func(t *testing.T) {
		cs := &fakeCollectionService{
			membership: map[string][]string{
				"alpha": {"col-other"},
				"beta":  {"col-other"},
				"gamma": {"col-other"},
			},
		}
		out, err := filterListingsByCollectionsWithService(context.Background(), cs, index, []string{"col-featured"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) != 0 {
			t.Fatalf("expected empty result, got %+v", out)
		}
	})

	t.Run("propagates service errors", func(t *testing.T) {
		sentinel := errors.New("db down")
		cs := &fakeCollectionService{err: sentinel}
		_, err := filterListingsByCollectionsWithService(context.Background(), cs, index, []string{"col-x"})
		if !errors.Is(err, sentinel) {
			t.Fatalf("expected sentinel error, got %v", err)
		}
	})

	t.Run("multiple collection ids treated as OR", func(t *testing.T) {
		cs := &fakeCollectionService{
			membership: map[string][]string{
				"alpha": {"col-a"},
				"beta":  {"col-b"},
				"gamma": {"col-c"},
			},
		}
		out, err := filterListingsByCollectionsWithService(context.Background(), cs, index, []string{"col-a", "col-b"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) != 2 {
			t.Fatalf("expected 2 listings (alpha, beta), got %d", len(out))
		}
	})
}
