package controller

import (
	"context"
	"fmt"

	"github.com/kagent-dev/kagent/go/core/internal/controller/apiclient"
	"github.com/kagent-dev/kagent/go/core/internal/substrate"
	"istio.io/istio/pkg/kube"
	"istio.io/istio/pkg/kube/krt"
	"k8s.io/client-go/rest"
)

// Runtime owns the Kubernetes client and common options shared by the v2 KRT
// collections. Collections must be created before Start so their informers are
// registered before the client starts.
type Runtime struct {
	Client      kube.Client
	Options     krt.OptionsBuilder
	Collections Collections
}

// RuntimeOption adjusts how the collection graph is built.
type RuntimeOption func(*runtimeConfig)

type runtimeConfig struct {
	actorPolicy substrate.ActorPolicy
}

// WithActorPolicy sets the operator's allowlist for Harness-requested actor
// adjustments. The default policy grants nothing.
func WithActorPolicy(policy substrate.ActorPolicy) RuntimeOption {
	return func(c *runtimeConfig) { c.actorPolicy = policy }
}

// NewRuntime creates the shared KRT client and collection graph. Handlers are
// deliberately registered separately at the eventual application boundary.
func NewRuntime(config *rest.Config, watchNamespaces []string, stop <-chan struct{}, opts ...RuntimeOption) (*Runtime, error) {
	var cfg runtimeConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	client, err := apiclient.New(config)
	if err != nil {
		return nil, fmt.Errorf("create KRT Kubernetes client: %w", err)
	}
	options := krt.NewOptionsBuilder(stop, "kagent", krt.GlobalDebugHandler)
	collections := NewCollections(client, watchNamespaces, cfg.actorPolicy, options)
	return &Runtime{Client: client, Options: options, Collections: collections}, nil
}

// Start starts every informer registered by the v2 KRT collections and keeps
// them alive until the application shuts down. The v2 API uses installed CRDs
// directly, so it does not start Istio's cluster-wide delayed-CRD watcher.
func (r *Runtime) Start(ctx context.Context) error {
	defer r.Client.Shutdown()
	if !r.Client.RunAndWait(ctx.Done()) {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("sync KRT Kubernetes client")
	}
	<-ctx.Done()
	return nil
}
