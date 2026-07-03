package repo

// defaultProductName is the product name when no brand.yaml is present.
// Override at build time via:
//
//	-ldflags "-X github.com/mobazha/mobazha/internal/repo.defaultProductName=ExampleMarket"
var defaultProductName = "Mobazha"
