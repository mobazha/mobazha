// Command gen-route-inventory records the effective Community HTTP surface
// after the real chi/Huma router composition has completed.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	internalapi "github.com/mobazha/mobazha3.0/internal/api"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/edition"
)

const schemaVersion = 1

type inventory struct {
	SchemaVersion int                           `json:"schemaVersion"`
	Edition       string                        `json:"edition"`
	Routes        []internalapi.RegisteredRoute `json:"routes"`
}

func main() {
	check := flag.Bool("check", false, "fail if the checked-in inventory differs")
	output := flag.String("output", filepath.Join("api-spec", "community-routes.json"), "inventory output path")
	flag.Parse()

	policy, err := edition.ResolvePolicy(edition.CommunityName)
	if err != nil {
		fail("resolve Community policy", err)
	}
	router, err := internalapi.NewSharedRouter(internalapi.SharedRouterConfig{
		Resolver:           func(*http.Request) (contracts.NodeService, error) { return nil, nil },
		DistributionPolicy: policy,
	})
	if err != nil {
		fail("build Community router", err)
	}
	routes, err := router.RegisteredRoutes()
	if err != nil {
		fail("walk Community router", err)
	}
	payload, err := json.MarshalIndent(inventory{
		SchemaVersion: schemaVersion,
		Edition:       policy.Name(),
		Routes:        routes,
	}, "", "  ")
	if err != nil {
		fail("encode route inventory", err)
	}
	payload = append(payload, '\n')

	if *check {
		existing, err := os.ReadFile(*output)
		if err != nil {
			fail("read checked-in route inventory", err)
		}
		if !bytes.Equal(existing, payload) {
			fmt.Fprintf(os.Stderr, "Community route inventory drift: %s; run make route-inventory\n", *output)
			os.Exit(1)
		}
		fmt.Printf("verified %s (%d routes)\n", *output, len(routes))
		return
	}

	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		fail("create inventory directory", err)
	}
	if err := os.WriteFile(*output, payload, 0o644); err != nil {
		fail("write route inventory", err)
	}
	fmt.Printf("wrote %s (%d routes)\n", *output, len(routes))
}

func fail(action string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}
