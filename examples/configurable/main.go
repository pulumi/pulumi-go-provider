// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"fmt"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// client is an interface to a (hypothetical) external system.
type client interface {
	CreateWidget(name string) (string, error)
}

// clientFactory is a function that creates a client based on the provider config.
// The config may include a connection string, credentials, or other parameters.
type clientFactory func(ctx context.Context, config Config) (client, error)

// newRealClient implements clientFactory and creates a real client based on the provider config.
func newRealClient(ctx context.Context, config Config) (client, error) {
	return &realClient{
		key:    config.ClientKey,
		secret: config.ClientSecret,
	}, nil
}

type realClient struct {
	key    string
	secret string
}

func (c *realClient) CreateWidget(name string) (string, error) {
	// In a real implementation, this would interact with an external system.
	return fmt.Sprintf("real-widget-%s-for-key-%s", name, c.key), nil
}

func main() {
	// Create a provider that uses the real client factory.
	provider, err := provider(newRealClient)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
	err = provider.Run(context.Background(), "configurable", "0.1.0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}

func provider(factory clientFactory) (p.Provider, error) {
	return infer.NewProviderBuilder().
		WithNamespace("examples").
		WithConfig(infer.Config(&Config{})).
		WithResources(
			infer.Resource(&Widget{getClient: factory}),
		).
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{
			"configurable": "index",
		}).
		Build()
}

type Config struct {
	ClientKey    string `pulumi:"clientKey"`
	ClientSecret string `pulumi:"clientSecret" provider:"secret"`
}

var _ = (infer.Annotated)((*Config)(nil))

func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(&c.ClientKey, "The client key to connect to the external system.")
	a.Describe(&c.ClientSecret, "The client secret to connect to the external system.")
}

type Widget struct {
	getClient clientFactory
}

type WidgetArgs struct{}
type WidgetState struct {
	Color string `pulumi:"color"`
}

func (w *Widget) Create(ctx context.Context, req infer.CreateRequest[WidgetArgs]) (infer.CreateResponse[WidgetState], error) {
	config := infer.GetConfig[Config](ctx)

	// Use the client factory to create a client based on the current config.
	client, err := w.getClient(ctx, config)
	if err != nil {
		return infer.CreateResponse[WidgetState]{}, err
	}

	// Use the client to create a widget.
	id, err := client.CreateWidget(req.Name)
	if err != nil {
		return infer.CreateResponse[WidgetState]{}, err
	}

	return infer.CreateResponse[WidgetState]{
		ID: id,
		Output: WidgetState{
			Color: "green",
		}}, nil
}
