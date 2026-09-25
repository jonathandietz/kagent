package translator

import (
	"strings"
	"testing"

	a2apb "github.com/a2aproject/a2a-go/v2/a2apb/v1"
	atev1alpha1 "github.com/agent-substrate/substrate/pkg/api/v1alpha1"
	"github.com/kagent-dev/kagent/go/api/v1alpha3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func TestRevisionDigestIncludesSandboxClass(t *testing.T) {
	revision := &Revision{Namespace: "agents", AgentTemplateName: "helper", HarnessName: "kagent"}
	original, err := revision.Digest()
	require.NoError(t, err)
	require.Equal(t, "3edf8e1756778ce192e3c834e6ebd8e2421dc23e9d64ade7d6ee3c6d6897cd6d", original.String())

	revision.SandboxClass = atev1alpha1.SandboxClassGvisor
	gvisor, err := revision.Digest()
	require.NoError(t, err)
	require.Equal(t, original, gvisor, "explicit gVisor must preserve the existing default revision")
	require.Equal(t, atev1alpha1.SandboxClassGvisor, revision.SandboxClass, "hashing must not mutate the revision")

	revision.SandboxClass = atev1alpha1.SandboxClassMicroVM
	microvm, err := revision.Digest()
	require.NoError(t, err)
	require.NotEqual(t, gvisor, microvm, "changing sandbox class must create a new immutable revision")
	repeated, err := revision.Digest()
	require.NoError(t, err)
	require.Equal(t, microvm, repeated)

	revision.SandboxClass = "unsupported"
	invalid, err := revision.Digest()
	require.EqualError(t, err, `unsupported sandbox class "unsupported"`)
	require.True(t, invalid.IsZero())
}

func TestRevisionDigestIncludesProvenance(t *testing.T) {
	revision := &Revision{Namespace: "agents", AgentTemplateName: "helper", HarnessName: "kagent", Provenance: []byte(`[{"kind":"ConfigMap","hash":"first"}]`)}
	first, err := revision.Digest()
	if err != nil {
		t.Fatal(err)
	}
	revision.Provenance = []byte(`[{"kind":"ConfigMap","hash":"second"}]`)
	second, err := revision.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("configuration change did not change runtime revision")
	}
	if len(first.Short()) != 12 || !strings.HasPrefix(first.String(), first.Short()) {
		t.Fatalf("short revision %q is not a prefix of %q", first.Short(), first.String())
	}
}

func TestRevisionDigestIncludesConfig(t *testing.T) {
	revision := &Revision{Namespace: "agents", AgentTemplateName: "helper", HarnessName: "claude", ConfigJSON: []byte(`{"version":5,"runtime_telemetry":{"capture_content":false}}`)}
	first, err := revision.Digest()
	if err != nil {
		t.Fatal(err)
	}
	revision.ConfigJSON = []byte(`{"version":5,"runtime_telemetry":{"capture_content":true}}`)
	second, err := revision.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("compiled configuration change did not change runtime revision")
	}
}

func TestCompilationWarningsDoNotAffectRevisionDigest(t *testing.T) {
	compilation := &CompileResult{Revision: Revision{Namespace: "agents", AgentTemplateName: "helper", HarnessName: "claude"}}
	first, err := compilation.Digest()
	if err != nil {
		t.Fatal(err)
	}
	compilation.Warnings = []string{"partial MCP selection is not enforced"}
	if len(compilation.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one warning", compilation.Warnings)
	}
	second, err := compilation.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("non-behavioral warning changed runtime revision")
	}
}

func TestRevisionDigestIncludesCommand(t *testing.T) {
	revision := &Revision{Namespace: "agents", AgentTemplateName: "helper", HarnessName: "byo", Command: []string{"/agent"}}
	first, err := revision.Digest()
	if err != nil {
		t.Fatal(err)
	}
	revision.Command = []string{"/other-agent"}
	second, err := revision.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("command change did not change runtime revision")
	}
}

func TestRevisionDigestIncludesBinaryAgentCard(t *testing.T) {
	card := &a2apb.AgentCard{Name: "assistant"}
	revision := &Revision{AgentCard: card}
	first, err := revision.Digest()
	require.NoError(t, err)
	card.Name = "changed"
	second, err := revision.Digest()
	require.NoError(t, err)
	require.NotEqual(t, first, second)
	card.ProtoReflect().SetUnknown(protowire.AppendString(protowire.AppendTag(nil, 1000, protowire.BytesType), "future"))
	third, err := revision.Digest()
	require.NoError(t, err)
	require.NotEqual(t, second, third)
	data, err := proto.Marshal(card)
	require.NoError(t, err)
	revision.AgentCard = &a2apb.AgentCard{}
	require.NoError(t, proto.Unmarshal(data, revision.AgentCard))
	roundTrip, err := revision.Digest()
	require.NoError(t, err)
	require.Equal(t, third, roundTrip)
	revision.AgentCard.Name = string([]byte{0xff})
	_, err = revision.Digest()
	require.Error(t, err)
}

func TestRevisionDigestIncludesCapabilities(t *testing.T) {
	revision := &Revision{Namespace: "agents", AgentTemplateName: "helper", HarnessName: "kagent"}
	original, err := revision.Digest()
	require.NoError(t, err)
	require.Equal(t, "3edf8e1756778ce192e3c834e6ebd8e2421dc23e9d64ade7d6ee3c6d6897cd6d", original.String(), "revisions without capabilities keep their identity")

	revision.Capabilities = &LinuxCapabilities{Add: []string{"SETFCAP"}}
	added, err := revision.Digest()
	require.NoError(t, err)
	require.NotEqual(t, original, added, "adding a capability must create a new immutable revision")
	require.Equal(t, &LinuxCapabilities{Add: []string{"SETFCAP"}}, revision.Capabilities, "hashing must not mutate the revision")

	revision.Capabilities = &LinuxCapabilities{Drop: []string{"SETFCAP"}}
	dropped, err := revision.Digest()
	require.NoError(t, err)
	require.NotEqual(t, added, dropped, "add and drop of the same name are different revisions")

	revision.Capabilities = nil
	restored, err := revision.Digest()
	require.NoError(t, err)
	require.Equal(t, original, restored, "removing the request restores the original digest")
}

func TestLinuxCapabilitiesFor(t *testing.T) {
	require.Nil(t, LinuxCapabilitiesFor(nil))
	require.Nil(t, LinuxCapabilitiesFor(&v1alpha3.HarnessSecurityContext{}))
	require.Nil(t, LinuxCapabilitiesFor(&v1alpha3.HarnessSecurityContext{Capabilities: &v1alpha3.HarnessLinuxCapabilities{}}), "an empty adjustment compiles like an omitted one")

	requested := &v1alpha3.HarnessSecurityContext{Capabilities: &v1alpha3.HarnessLinuxCapabilities{Add: []string{"SETFCAP"}, Drop: []string{"NET_BIND_SERVICE"}}}
	capabilities := LinuxCapabilitiesFor(requested)
	require.Equal(t, &LinuxCapabilities{Add: []string{"SETFCAP"}, Drop: []string{"NET_BIND_SERVICE"}}, capabilities)
	capabilities.Add[0] = "changed"
	require.Equal(t, "SETFCAP", requested.Capabilities.Add[0], "the revision must not alias the public object")
}
