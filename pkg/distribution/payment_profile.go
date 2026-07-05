// SPDX-License-Identifier: MPL-2.0

package distribution

import (
	"fmt"
	"strings"
)

// PaymentProfileRequirement determines whether a distribution may start when
// a selected trusted module is absent from the statically linked binary.
type PaymentProfileRequirement string

const (
	PaymentProfileRequired PaymentProfileRequirement = "required"
	PaymentProfileOptional PaymentProfileRequirement = "optional"
)

// PaymentProfileModule selects one trusted module by stable descriptor ID.
type PaymentProfileModule struct {
	ModuleID    string
	Requirement PaymentProfileRequirement
}

// PaymentModuleProfile is an immutable composition-time selection. It does not
// install code or own runtime state; it chooses among reviewed modules already
// linked into the distribution.
type PaymentModuleProfile struct {
	ID      string
	Version string
	Modules []PaymentProfileModule
}

// SelectPaymentModules validates a profile and returns the selected module
// instances in profile order. Available-but-unselected modules are deliberately
// omitted; construction remains the composition root's responsibility.
func SelectPaymentModules(profile PaymentModuleProfile, available []PaymentModule) ([]PaymentModule, error) {
	profile.ID = strings.TrimSpace(profile.ID)
	profile.Version = strings.TrimSpace(profile.Version)
	if profile.ID == "" || profile.Version == "" {
		return nil, fmt.Errorf("payment module profile ID and version are required")
	}
	byID := make(map[string]PaymentModule, len(available))
	for index, module := range available {
		if isNilInterface(module) {
			return nil, fmt.Errorf("payment module profile %q available module %d is nil", profile.ID, index)
		}
		descriptor := normalizedPaymentModuleDescriptor(module.Descriptor())
		if err := validatePaymentModuleDescriptor(descriptor); err != nil {
			return nil, err
		}
		if _, duplicate := byID[descriptor.ID]; duplicate {
			return nil, fmt.Errorf("payment module profile %q has duplicate available module %q", profile.ID, descriptor.ID)
		}
		byID[descriptor.ID] = module
	}

	selected := make([]PaymentModule, 0, len(profile.Modules))
	seen := make(map[string]struct{}, len(profile.Modules))
	for _, entry := range profile.Modules {
		entry.ModuleID = strings.TrimSpace(entry.ModuleID)
		if entry.ModuleID == "" {
			return nil, fmt.Errorf("payment module profile %q contains an empty module ID", profile.ID)
		}
		if _, duplicate := seen[entry.ModuleID]; duplicate {
			return nil, fmt.Errorf("payment module profile %q selects module %q more than once", profile.ID, entry.ModuleID)
		}
		seen[entry.ModuleID] = struct{}{}
		switch entry.Requirement {
		case PaymentProfileRequired, PaymentProfileOptional:
		default:
			return nil, fmt.Errorf("payment module profile %q module %q has invalid requirement %q", profile.ID, entry.ModuleID, entry.Requirement)
		}
		module, exists := byID[entry.ModuleID]
		if !exists {
			if entry.Requirement == PaymentProfileRequired {
				return nil, fmt.Errorf("payment module profile %q requires unavailable module %q", profile.ID, entry.ModuleID)
			}
			continue
		}
		selected = append(selected, module)
	}
	return selected, nil
}
