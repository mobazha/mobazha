package api

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	crypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

type TransactionQuery struct {
	OrderStates     []int    `json:"states"`
	SearchTerm      string   `json:"search"`
	SortByAscending bool     `json:"sortByAscending"`
	SortByRead      bool     `json:"sortByRead"`
	Limit           int      `json:"limit"`
	Exclude         []string `json:"exclude"`
}

func parseSearchTerms(q url.Values) (orderStates []models.OrderState, searchTerm string, sortByAscending, sortByRead bool, limit int, err error) {
	limitStr := q.Get("limit")
	if limitStr == "" {
		limitStr = "-1"
	}
	limit, err = strconv.Atoi(limitStr)
	if err != nil {
		return orderStates, searchTerm, false, false, 0, err
	}

	stateQuery := q.Get("state")
	states := strings.Split(stateQuery, ",")
	for _, s := range states {
		if s != "" {
			i, err := strconv.Atoi(s)
			if err != nil {
				return orderStates, searchTerm, false, false, 0, err
			}
			orderStates = append(orderStates, models.OrderState(i))
		}
	}

	searchTerm = q.Get("search")
	sortTerms := strings.Split(q.Get("sortBy"), ",")
	if len(sortTerms) > 0 {
		for _, term := range sortTerms {
			switch strings.ToLower(term) {
			case "data-asc":
				sortByAscending = true
			case "read":
				sortByRead = true
			}
		}
	}
	return orderStates, searchTerm, sortByAscending, sortByRead, limit, nil
}

func convertOrderStates(states []int) []models.OrderState {
	var orderStates []models.OrderState
	for _, i := range states {
		orderStates = append(orderStates, models.OrderState(i))
	}
	return orderStates
}

var ErrInvalidKey = errors.New("invalid key")

// signPayload produces a signature for the given private key and payload
// and returns it and the public key or an error
func signPayload(payload []byte, privKey crypto.PrivKey) ([]byte, []byte, error) {
	if privKey == nil {
		return nil, nil, ErrInvalidKey
	}
	var (
		sig, sErr    = privKey.Sign(payload)
		pubkey, pErr = privKey.GetPublic().Raw()
	)
	if sErr != nil {
		return nil, nil, fmt.Errorf("signing payload: %s", sErr.Error())
	}
	if pErr != nil {
		return nil, nil, fmt.Errorf("getting pub key: %s", pErr.Error())
	}
	return sig, pubkey, nil
}

// verifyPayload proves the payload and signature are authentic
// for the provided public key and returns the peer ID for that
// pubkey with no error on success
func verifyPayload(payload, sig, pubKey []byte) (string, error) {
	if len(pubKey) == 0 {
		return "", ErrInvalidKey
	}
	pk, err := crypto.UnmarshalPublicKey(pubKey)
	if err != nil {
		return "", err
	}

	_, err = pk.Verify(payload, sig)
	if err != nil {
		return "", err
	}

	peerID, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return "", err
	}

	return peerID.String(), nil
}
