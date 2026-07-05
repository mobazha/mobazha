package distribution

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/mobazha/mobazha/pkg/contracts"
)

const (
	trustedOwnerExtension       = "x-mobazha-module-owner"
	trustedAuthExtension        = "x-mobazha-auth-profile"
	trustedAPITokenExtension    = "x-mobazha-api-token-scope"
	trustedOperationOwnerKey    = "mobazha.trusted-module-owner"
	trustedOperationAuthKey     = "mobazha.trusted-auth-profile"
	trustedOperationAPIScopeKey = "mobazha.trusted-api-token-scope"
)

var trustedModuleOwnerPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)

// TrustedHumaModuleDescriptor declares the least-privilege HTTP namespace a
// build-time distribution module may contribute. Namespaces and operation ID
// prefixes are grants, not documentation: registration outside them fails.
type TrustedHumaModuleDescriptor struct {
	Owner               string
	PathNamespaces      []string
	OperationIDPrefixes []string
}

// TrustedHumaRegistrationConfig is used by the Core-owned gateway to construct
// a restricted module registration capability. Distribution modules receive
// only the resulting TrustedHumaRegistration value, never the raw Huma API.
type TrustedHumaRegistrationConfig struct {
	API                   huma.API
	Descriptor            TrustedHumaModuleDescriptor
	NodeAuthSecurity      []map[string][]string
	AdminOnlySecurity     []map[string][]string
	ValidateAPITokenScope func(method, path string, scope contracts.Scope) error
}

type trustedHumaRegistrar struct {
	api                   huma.API
	descriptor            TrustedHumaModuleDescriptor
	nodeAuthSecurity      []map[string][]string
	adminOnlySecurity     []map[string][]string
	validateAPITokenScope func(method, path string, scope contracts.Scope) error
}

// TrustedHumaRegistration is the narrow in-process API capability available
// to first-party distribution modules. Operations can be added only through
// the typed RegisterTrusted* helpers below.
type TrustedHumaRegistration struct {
	registrar *trustedHumaRegistrar
}

// NewTrustedHumaRegistration constructs a least-privilege registration
// capability for one module owner.
func NewTrustedHumaRegistration(config TrustedHumaRegistrationConfig) (TrustedHumaRegistration, error) {
	if config.API == nil {
		return TrustedHumaRegistration{}, fmt.Errorf("trusted Huma registration: API is required")
	}
	descriptor, err := validateTrustedHumaModuleDescriptor(config.Descriptor)
	if err != nil {
		return TrustedHumaRegistration{}, err
	}
	if config.ValidateAPITokenScope == nil {
		return TrustedHumaRegistration{}, fmt.Errorf("trusted Huma registration %q: API token scope validator is required", descriptor.Owner)
	}
	if len(config.NodeAuthSecurity) == 0 || len(config.AdminOnlySecurity) == 0 {
		return TrustedHumaRegistration{}, fmt.Errorf("trusted Huma registration %q: gateway security requirements are required", descriptor.Owner)
	}
	return TrustedHumaRegistration{registrar: &trustedHumaRegistrar{
		api:                   config.API,
		descriptor:            descriptor,
		nodeAuthSecurity:      cloneTrustedSecurity(config.NodeAuthSecurity),
		adminOnlySecurity:     cloneTrustedSecurity(config.AdminOnlySecurity),
		validateAPITokenScope: config.ValidateAPITokenScope,
	}}, nil
}

// RegisterTrustedAdminOnly registers an operation that accepts only full
// administrator identities. Scoped API tokens are intentionally excluded.
func RegisterTrustedAdminOnly[I, O any](
	registration TrustedHumaRegistration,
	operation huma.Operation,
	handler func(context.Context, *I) (*O, error),
) error {
	return registerTrustedOperation(registration, operation, "admin-only", "", handler)
}

// RegisterTrustedNodeToken registers an operation that accepts normal node
// identities, including a scoped API token. The declared scope must match the
// Core route-scope policy or registration fails.
func RegisterTrustedNodeToken[I, O any](
	registration TrustedHumaRegistration,
	operation huma.Operation,
	scope contracts.Scope,
	handler func(context.Context, *I) (*O, error),
) error {
	if strings.TrimSpace(string(scope)) == "" {
		return fmt.Errorf("trusted Huma operation %q: API token scope is required", operation.OperationID)
	}
	return registerTrustedOperation(registration, operation, "node-token", scope, handler)
}

// RegisterTrustedPublic registers an explicitly unauthenticated operation.
func RegisterTrustedPublic[I, O any](
	registration TrustedHumaRegistration,
	operation huma.Operation,
	handler func(context.Context, *I) (*O, error),
) error {
	return registerTrustedOperation(registration, operation, "public", "", handler)
}

// TrustedOperationAPITokenScope returns the explicit API-token scope attached
// by RegisterTrustedNodeToken.
func TrustedOperationAPITokenScope(operation *huma.Operation) (contracts.Scope, bool) {
	if operation == nil || operation.Metadata == nil {
		return "", false
	}
	value, ok := operation.Metadata[trustedOperationAPIScopeKey].(contracts.Scope)
	return value, ok && value != ""
}

func registerTrustedOperation[I, O any](
	registration TrustedHumaRegistration,
	operation huma.Operation,
	authProfile string,
	scope contracts.Scope,
	handler func(context.Context, *I) (*O, error),
) error {
	registrar := registration.registrar
	if registrar == nil || registrar.api == nil {
		return fmt.Errorf("trusted Huma operation %q: invalid registration capability", operation.OperationID)
	}
	if handler == nil {
		return fmt.Errorf("trusted Huma operation %q: handler is required", operation.OperationID)
	}
	if operation.Security != nil {
		return fmt.Errorf("trusted Huma operation %q: security is owned by the gateway", operation.OperationID)
	}
	operation.OperationID = strings.TrimSpace(operation.OperationID)
	operation.Method = strings.ToUpper(strings.TrimSpace(operation.Method))
	operation.Path = strings.TrimSpace(operation.Path)
	if err := registrar.validateOperation(operation); err != nil {
		return err
	}

	switch authProfile {
	case "admin-only":
		operation.Security = cloneTrustedSecurity(registrar.adminOnlySecurity)
	case "node-token":
		if err := registrar.validateAPITokenScope(operation.Method, operation.Path, scope); err != nil {
			return fmt.Errorf("trusted Huma operation %q: %w", operation.OperationID, err)
		}
		operation.Security = cloneTrustedSecurity(registrar.nodeAuthSecurity)
	case "public":
		operation.Security = []map[string][]string{}
	default:
		return fmt.Errorf("trusted Huma operation %q: unsupported auth profile %q", operation.OperationID, authProfile)
	}

	if operation.Metadata == nil {
		operation.Metadata = make(map[string]any)
	}
	if operation.Extensions == nil {
		operation.Extensions = make(map[string]any)
	}
	operation.Metadata[trustedOperationOwnerKey] = registrar.descriptor.Owner
	operation.Metadata[trustedOperationAuthKey] = authProfile
	operation.Extensions[trustedOwnerExtension] = registrar.descriptor.Owner
	operation.Extensions[trustedAuthExtension] = authProfile
	if scope != "" {
		operation.Metadata[trustedOperationAPIScopeKey] = scope
		operation.Extensions[trustedAPITokenExtension] = string(scope)
	}

	huma.Register(registrar.api, operation, handler)
	return nil
}

func (registrar *trustedHumaRegistrar) validateOperation(operation huma.Operation) error {
	operationID := strings.TrimSpace(operation.OperationID)
	if operationID == "" {
		return fmt.Errorf("trusted Huma module %q: operation ID is required", registrar.descriptor.Owner)
	}
	if !hasAllowedPrefix(operationID, registrar.descriptor.OperationIDPrefixes, false) {
		return fmt.Errorf("trusted Huma operation %q: operation ID is outside module %q prefixes", operationID, registrar.descriptor.Owner)
	}
	method := strings.ToUpper(strings.TrimSpace(operation.Method))
	if method == "" {
		return fmt.Errorf("trusted Huma operation %q: method is required", operationID)
	}
	switch method {
	case http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete,
		http.MethodOptions, http.MethodHead, http.MethodPatch, http.MethodTrace:
	default:
		return fmt.Errorf("trusted Huma operation %q: method %q is unsupported", operationID, method)
	}
	if operation.Hidden {
		return fmt.Errorf("trusted Huma operation %q: hidden operations are not allowed", operationID)
	}
	if operation.Path == "" || !strings.HasPrefix(operation.Path, "/v1/") {
		return fmt.Errorf("trusted Huma operation %q: path must be under /v1", operationID)
	}
	if !hasAllowedPrefix(operation.Path, registrar.descriptor.PathNamespaces, true) {
		return fmt.Errorf("trusted Huma operation %q: path %q is outside module %q namespaces", operationID, operation.Path, registrar.descriptor.Owner)
	}
	if operation.Extensions != nil {
		for _, key := range []string{trustedOwnerExtension, trustedAuthExtension, trustedAPITokenExtension} {
			if _, exists := operation.Extensions[key]; exists {
				return fmt.Errorf("trusted Huma operation %q: extension %q is gateway-owned", operationID, key)
			}
		}
	}
	if operation.Metadata != nil {
		for _, key := range []string{trustedOperationOwnerKey, trustedOperationAuthKey, trustedOperationAPIScopeKey} {
			if _, exists := operation.Metadata[key]; exists {
				return fmt.Errorf("trusted Huma operation %q: metadata %q is gateway-owned", operationID, key)
			}
		}
	}

	for path, item := range registrar.api.OpenAPI().Paths {
		for existingMethod, existing := range pathItemOperations(item) {
			if existing == nil {
				continue
			}
			if existing.OperationID == operationID {
				return fmt.Errorf("trusted Huma operation %q: duplicate operation ID at %s %s", operationID, strings.ToUpper(existingMethod), path)
			}
			if path == operation.Path && strings.EqualFold(existingMethod, method) {
				return fmt.Errorf("trusted Huma operation %q: duplicate route %s %s", operationID, method, operation.Path)
			}
		}
	}
	return nil
}

func validateTrustedHumaModuleDescriptor(descriptor TrustedHumaModuleDescriptor) (TrustedHumaModuleDescriptor, error) {
	descriptor.Owner = strings.TrimSpace(descriptor.Owner)
	if !trustedModuleOwnerPattern.MatchString(descriptor.Owner) {
		return TrustedHumaModuleDescriptor{}, fmt.Errorf("trusted Huma module owner %q is invalid", descriptor.Owner)
	}
	if len(descriptor.PathNamespaces) == 0 || len(descriptor.OperationIDPrefixes) == 0 {
		return TrustedHumaModuleDescriptor{}, fmt.Errorf("trusted Huma module %q requires path and operation ID grants", descriptor.Owner)
	}
	descriptor.PathNamespaces = append([]string(nil), descriptor.PathNamespaces...)
	descriptor.OperationIDPrefixes = append([]string(nil), descriptor.OperationIDPrefixes...)
	for _, namespace := range descriptor.PathNamespaces {
		if !strings.HasPrefix(namespace, "/v1/") || strings.Contains(namespace, "{") {
			return TrustedHumaModuleDescriptor{}, fmt.Errorf("trusted Huma module %q has invalid path namespace %q", descriptor.Owner, namespace)
		}
	}
	for _, prefix := range descriptor.OperationIDPrefixes {
		if strings.TrimSpace(prefix) == "" {
			return TrustedHumaModuleDescriptor{}, fmt.Errorf("trusted Huma module %q has an empty operation ID prefix", descriptor.Owner)
		}
	}
	return descriptor, nil
}

func hasAllowedPrefix(value string, prefixes []string, path bool) bool {
	for _, prefix := range prefixes {
		if path {
			if value == prefix || strings.HasPrefix(value, strings.TrimRight(prefix, "/")+"/") {
				return true
			}
			continue
		}
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func pathItemOperations(item *huma.PathItem) map[string]*huma.Operation {
	if item == nil {
		return nil
	}
	return map[string]*huma.Operation{
		http.MethodGet:     item.Get,
		http.MethodPut:     item.Put,
		http.MethodPost:    item.Post,
		http.MethodDelete:  item.Delete,
		http.MethodOptions: item.Options,
		http.MethodHead:    item.Head,
		http.MethodPatch:   item.Patch,
		http.MethodTrace:   item.Trace,
	}
}

func cloneTrustedSecurity(requirements []map[string][]string) []map[string][]string {
	if requirements == nil {
		return nil
	}
	clone := make([]map[string][]string, len(requirements))
	for index, requirement := range requirements {
		clone[index] = make(map[string][]string, len(requirement))
		for scheme, scopes := range requirement {
			copyScopes := make([]string, len(scopes))
			copy(copyScopes, scopes)
			clone[index][scheme] = copyScopes
		}
	}
	return clone
}

// TrustedHumaModule registers first-party routes into the Core-owned gateway.
// It is a build-time composition contract, not the third-party plugin API.
type TrustedHumaModule interface {
	TrustedHumaModuleDescriptor() TrustedHumaModuleDescriptor
	RegisterTrustedHuma(registration TrustedHumaRegistration) error
}
