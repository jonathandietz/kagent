package substrate

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kagent-dev/kagent/go/core/internal/translator"
)

// ActorPolicy is the operator's allowlist for what a Harness may change about
// its actor container. Harness authors request; the controller's operator
// decides what is grantable, so a namespace-scoped resource cannot widen its
// own sandbox.
type ActorPolicy struct {
	// AllowedCapabilities lists the capabilities a Harness may add, named
	// without the CAP_ prefix. Empty grants nothing.
	AllowedCapabilities []string
}

// ParseAllowedCapabilities reads a comma-separated allowlist as the controller
// receives it from its environment. Entries are trimmed, empty entries are
// ignored, and the CAP_ prefix is accepted and removed so a copied kernel
// spelling still names the capability Substrate expects.
func ParseAllowedCapabilities(raw string) []string {
	var allowed []string
	for entry := range strings.SplitSeq(raw, ",") {
		entry = strings.TrimPrefix(strings.TrimSpace(entry), "CAP_")
		if entry == "" || slices.Contains(allowed, entry) {
			continue
		}
		allowed = append(allowed, entry)
	}
	return allowed
}

// CapabilityNotAllowedError names the first requested addition the policy
// refuses.
type CapabilityNotAllowedError struct {
	Capability string
}

func (e *CapabilityNotAllowedError) Error() string {
	return fmt.Sprintf("capability %q is not in the controller's actor capability allowlist", e.Capability)
}

// Check refuses a capability addition the policy does not allow. Drops are
// always permitted: removing a default capability never widens the sandbox.
func (p ActorPolicy) Check(capabilities *translator.LinuxCapabilities) error {
	if capabilities == nil {
		return nil
	}
	for _, capability := range capabilities.Add {
		if !slices.Contains(p.AllowedCapabilities, capability) {
			return &CapabilityNotAllowedError{Capability: capability}
		}
	}
	return nil
}
