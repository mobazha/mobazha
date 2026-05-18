package relay

import (
	"os"
	"strings"
)

// EnvPlatformRelayToken is read when RelayAPIBearer / config bearer is empty (Docker/legacy).
const EnvPlatformRelayToken = "MOBAZHA_PLATFORM_RELAY_TOKEN"

// BearerFromConfigOrEnv returns trimmed configured Bearer JWT, or the platform-relay
// env token when empty. Matches standalone ManagedEscrow + Settlement HTTP relay wiring.
func BearerFromConfigOrEnv(configured string) string {
	b := strings.TrimSpace(configured)
	if b != "" {
		return b
	}
	return strings.TrimSpace(os.Getenv(EnvPlatformRelayToken))
}
