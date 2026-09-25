/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha3

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl_client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// TestHarnessWorkloadSecurityContextValidation pins the admission rules for
// spec.workload.securityContext.capabilities against a real kube-apiserver
// loaded with the shipped CRDs. Capabilities are named without the CAP_
// prefix, as Substrate's ActorTemplate API expects; ALL is accepted only in
// drop; each list is a set of uppercase names.
func TestHarnessWorkloadSecurityContextValidation(t *testing.T) {
	testEnv := &envtest.Environment{
		BinaryAssetsDirectory: envtestAssetsDir(t),
		CRDDirectoryPaths:     []string{crdBasesDir(t)},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := testEnv.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = testEnv.Stop() })

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, AddToScheme(scheme))
	cl, err := ctrl_client.New(cfg, ctrl_client.Options{Scheme: scheme})
	require.NoError(t, err)

	ctx := context.Background()
	const ns = "harness-cel"
	require.NoError(t, cl.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}))

	harness := func(name string, capabilities *HarnessLinuxCapabilities) *Harness {
		return &Harness{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: HarnessSpec{
				Kagent: &KagentHarness{},
				Workload: HarnessWorkload{
					Image:           "example.com/agent@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SecurityContext: &HarnessSecurityContext{Capabilities: capabilities},
				},
				Substrate: HarnessSubstratePolicy{
					WorkerPoolRef:  corev1.LocalObjectReference{Name: "default"},
					SnapshotPolicy: HarnessSnapshotPolicy{Location: "snapshots"},
				},
			},
		}
	}

	cases := []struct {
		name         string
		capabilities *HarnessLinuxCapabilities
		wantReject   string // substring in admission error; empty means accept
	}{
		{name: "add and drop accepted", capabilities: &HarnessLinuxCapabilities{Add: []string{"SETFCAP", "SETUID"}, Drop: []string{"NET_BIND_SERVICE"}}},
		{name: "drop ALL accepted", capabilities: &HarnessLinuxCapabilities{Drop: []string{"ALL"}}},
		{name: "empty adjustment accepted", capabilities: &HarnessLinuxCapabilities{}},
		{name: "add ALL rejected", capabilities: &HarnessLinuxCapabilities{Add: []string{"ALL"}}, wantReject: "add does not accept ALL"},
		{name: "CAP_ prefix rejected in add", capabilities: &HarnessLinuxCapabilities{Add: []string{"CAP_SETFCAP"}}, wantReject: "without the CAP_ prefix"},
		{name: "CAP_ prefix rejected in drop", capabilities: &HarnessLinuxCapabilities{Drop: []string{"CAP_KILL"}}, wantReject: "without the CAP_ prefix"},
		{name: "lowercase rejected", capabilities: &HarnessLinuxCapabilities{Add: []string{"setfcap"}}, wantReject: "should match"},
		{name: "duplicate rejected", capabilities: &HarnessLinuxCapabilities{Add: []string{"SETFCAP", "SETFCAP"}}, wantReject: "Duplicate value"},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj := harness("h-"+string(rune('a'+i)), tc.capabilities)
			err := cl.Create(ctx, obj)
			if tc.wantReject == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantReject)
		})
	}
}
