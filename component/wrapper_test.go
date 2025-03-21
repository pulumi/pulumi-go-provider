// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package component

import (
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockComponentResource is a minimal implementation of pulumi.ComponentResource for testing
type MockComponentResource struct {
	pulumi.ResourceState
	MockComponentResourceInput
}

// MockComponentResourceInput is a minimal implementation of the input struct for testing
type MockComponentResourceInput struct{}

// NewMockComponentResource creates a new instance of MockComponentResource
func NewMockComponentResource(
	ctx *pulumi.Context,
	name string,
	inputs MockComponentResourceInput,
	options ...pulumi.ResourceOption) (*MockComponentResource, error) {
	return &MockComponentResource{}, nil
}

// TestProvider tests that provider returns a non-nil provider
func TestProvider(t *testing.T) {
	t.Parallel()
	p := provider(nil)
	require.NotNil(t, p)
	require.Nil(t, p.Construct, "construct method should be nil if no components are registered")

	// Create a provider with resources and ensure the construct method is not nil.
	components := map[Resource]struct{}{
		ProgramComponent(NewMockComponentResource): {}}
	p = provider(components)
	assert.NotNil(t, p.Construct)
}

func TestProviderHost(t *testing.T) {
	t.Parallel()
	// Create a provider without any components and ensure that it returns an error.
	err := ProviderHost()
	assert.Error(t, err)
	assert.Equal(t, "no resource components were registered with the provider", err.Error())

	go func() {
		// Start the provider in a separate goroutine as it blocks the main thread.
		err := ProviderHost(WithResources(ProgramComponent(NewMockComponentResource)))
		require.NoError(t, err)
	}()

	time.Sleep(2 * time.Second) // Wait for the provider to start to ensure it doesn't error.
}
func TestProviderOptions(t *testing.T) {
	t.Parallel()

	// Test WithResource
	t.Run("WithResource", func(t *testing.T) {
		opts := &providerOpts{
			components: make(map[Resource]struct{}),
		}
		resource := ProgramComponent(NewMockComponentResource)
		WithResources(resource)(opts)

		_, exists := opts.components[resource]
		assert.True(t, exists)
		assert.Len(t, opts.components, 1)
	})

	// Test WithName
	t.Run("WithName", func(t *testing.T) {
		opts := &providerOpts{}
		WithName("test-provider")(opts)

		assert.Equal(t, "test-provider", opts.name)
	})

	// Test WithVersion
	t.Run("WithVersion", func(t *testing.T) {
		opts := &providerOpts{}
		WithVersion("1.0.0")(opts)

		assert.Equal(t, "1.0.0", opts.version)
	})

	// Test multiple options together
	t.Run("MultipleOptions", func(t *testing.T) {
		opts := &providerOpts{
			components: make(map[Resource]struct{}),
		}
		resource := ProgramComponent(NewMockComponentResource)

		WithResources(resource)(opts)
		WithName("test-provider")(opts)
		WithVersion("1.0.0")(opts)

		_, exists := opts.components[resource]
		assert.True(t, exists)
		assert.Equal(t, "test-provider", opts.name)
		assert.Equal(t, "1.0.0", opts.version)
	})
}
