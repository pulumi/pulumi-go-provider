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

func createMockInferredComoponent() Resource {
	// Reset the global registry before the test
	globalState = state{inferredComponents: make(map[Resource]struct{})}

	// Create a mock component
	return ProgramComponent(NewMockComponentResource)
}

// TestRegisterType tests the RegisterType function
//
//nolint:paralleltest
func TestRegisterType(t *testing.T) {
	// Create a mock component
	mockComponent := createMockInferredComoponent()

	// Register the component
	RegisterType(mockComponent)

	// Verify the component was registered
	assert.Equal(t, 1, len(globalState.inferredComponents))
	assert.Contains(t, globalState.inferredComponents, mockComponent)

	// Register the same component again, and verify it doesn't duplicate.
	RegisterType(mockComponent)
	assert.Equal(t, 1, len(globalState.inferredComponents))
	assert.Contains(t, globalState.inferredComponents, mockComponent)
}

// TestProvider tests that provider returns a non-nil provider
//
//nolint:paralleltest
func TestProvider(t *testing.T) {
	// Create a provider without any components and ensure that it returns a nil construct method.
	globalState = state{inferredComponents: make(map[Resource]struct{})}
	require.Empty(t, globalState.inferredComponents)
	p := provider()
	require.NotNil(t, p)
	require.Nil(t, p.Construct, "Construct method: %+v", p.Construct)

	// Create and register a mock component.
	mockComponent := createMockInferredComoponent()
	RegisterType(mockComponent)

	// Create a provider and ensure the construct method is not nil.
	p = provider()
	assert.NotNil(t, p.Construct)
}

//nolint:paralleltest
func TestProviderHost(t *testing.T) {
	// Create a provider without any components and ensure that it returns an error.
	globalState = state{inferredComponents: make(map[Resource]struct{})}
	err := ProviderHost("test", "v1.0.0")
	assert.Error(t, err)
	assert.Equal(t, "no resource components were registered with the provider", err.Error())

	// Create and register a mock component and reset the state prior.
	globalState = state{inferredComponents: make(map[Resource]struct{})}
	mockComponent := createMockInferredComoponent()
	RegisterType(mockComponent)

	go func() {
		// Start the provider in a separate goroutine as it blocks the main thread.
		err := ProviderHost("test", "v1.0.0")
		require.NoError(t, err)
	}()

	time.Sleep(1 * time.Millisecond) // Wait for the provider to start
	require.PanicsWithValue(t, "provider has already started; cannot register new types", func() {
		RegisterType(mockComponent)
	})

	// Attempt to start the provider again and ensure it returns an error.
	err = ProviderHost("test", "v1.0.0")
	require.Error(t, err)
	require.Equal(t, "provider had already started", err.Error())
}
