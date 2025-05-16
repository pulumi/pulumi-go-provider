// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"testing"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	integration "github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

//go:generate go tool mockgen -typed -package main -destination mocks.gen.go -imports goprovider=github.com/pulumi/pulumi-go-provider . client

func TestWidget(t *testing.T) {

	// This test demonstrates the use of mock client injection for testing your provider code.
	// Here we configure a mock client to return a fake widget ID when CreateWidget is called.
	ctrl := gomock.NewController(t)
	mockClient := NewMockclient(ctrl)
	mockClient.EXPECT().CreateWidget(gomock.Any()).DoAndReturn(func(name string) (string, error) {
		return "fake-widget-id", nil
	}).AnyTimes()

	// Create the provider such that it uses the mock client.
	newMockClient := func(ctx context.Context, config Config) (client, error) {
		assert.Equal(t, "mykey", config.ClientKey)
		assert.Equal(t, "mysecret", config.ClientSecret)
		return mockClient, nil
	}
	provider, err := provider(newMockClient)
	require.NoError(t, err)

	// Create the integration server.
	server, err := integration.NewServer(t.Context(),
		"configurable",
		semver.Version{Minor: 1},
		integration.WithProvider(provider),
	)
	require.NoError(t, err)

	// Configure the provider with a fake client key and secret.
	server.Configure(p.ConfigureRequest{
		Args: property.NewMap(map[string]property.Value{
			"clientKey":    property.New("mykey"),
			"clientSecret": property.New("mysecret").WithSecret(true),
		}),
	})

	// Test the lifecycle methods of the Widget resource, expecting it to use the mock client.
	integration.LifeCycleTest{
		Resource: "configurable:index:Widget",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{}),
			Hook: func(inputs, output property.Map) {
				t.Logf("Outputs: %#v", output)
				color := output.Get("color").AsString()
				assert.Equal(t, "green", color)
			},
		},
	}.Run(t, server)
}
