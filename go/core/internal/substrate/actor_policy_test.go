package substrate

import (
	"testing"

	"github.com/kagent-dev/kagent/go/core/internal/translator"
	"github.com/stretchr/testify/require"
)

func TestParseAllowedCapabilities(t *testing.T) {
	require.Nil(t, ParseAllowedCapabilities(""))
	require.Nil(t, ParseAllowedCapabilities(" , ,"))
	require.Equal(t, []string{"SETFCAP", "SETUID", "SETGID"}, ParseAllowedCapabilities(" SETFCAP,CAP_SETUID , SETGID,SETFCAP,"))
}

func TestActorPolicyCheck(t *testing.T) {
	policy := ActorPolicy{AllowedCapabilities: []string{"SETFCAP"}}
	require.NoError(t, ActorPolicy{}.Check(nil), "no request needs no allowance")
	require.NoError(t, policy.Check(&translator.LinuxCapabilities{Add: []string{"SETFCAP"}}))
	require.NoError(t, ActorPolicy{}.Check(&translator.LinuxCapabilities{Drop: []string{"ALL"}}), "drops never widen the sandbox")

	err := ActorPolicy{}.Check(&translator.LinuxCapabilities{Add: []string{"SETFCAP"}})
	var refused *CapabilityNotAllowedError
	require.ErrorAs(t, err, &refused)
	require.Equal(t, "SETFCAP", refused.Capability)
	require.EqualError(t, err, `capability "SETFCAP" is not in the controller's actor capability allowlist`)

	err = policy.Check(&translator.LinuxCapabilities{Add: []string{"SETFCAP", "SYS_ADMIN"}, Drop: []string{"KILL"}})
	require.ErrorAs(t, err, &refused)
	require.Equal(t, "SYS_ADMIN", refused.Capability, "the first refused addition is named")
}
