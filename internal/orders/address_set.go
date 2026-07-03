package orders

import (
	"strings"

	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

type addressSet struct {
	addrs      map[string]struct{}
	caseInsens bool
}

func newAddressSet(coin string) *addressSet {
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(coin))
	caseInsens := err == nil && coinInfo.IsEthTypeChain()
	return &addressSet{
		addrs:      make(map[string]struct{}),
		caseInsens: caseInsens,
	}
}

func (s *addressSet) Add(addr string) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return
	}
	if s.caseInsens {
		addr = strings.ToLower(addr)
	}
	s.addrs[addr] = struct{}{}
}

func (s *addressSet) Contains(addr string) bool {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return false
	}
	if s.caseInsens {
		addr = strings.ToLower(addr)
	}
	_, ok := s.addrs[addr]
	return ok
}

func isZeroAmount(amount string) bool {
	amount = strings.TrimSpace(amount)
	return amount == "" || amount == "0"
}
