# Integration


The `integration` package provides a library for writing integration tests for your provider code. It enables testing provider behavior in-memory, sitting just above the gRPC level. This package is particularly useful for validating the lifecycle of resources and ensuring correctness in provider implementations.

## Server

The package includes a `Server` interface and its implementation, which acts as a test harness for Pulumi providers.
The interface allows you to exercise the RPC methods of your provider, including:
- Schema retrieval
- Provider configuration
- Resource lifecycle operations (create, read, update, delete)
- Component lifecycle operations (construct)
- Provider functions (invoke) and resource functions (call)

To make a server, call `integration.NewServer` and use `WithProvider` to pass the `Provider` instance to be tested.
Then, exercise the various RPC methods of your provider.

## Component Resource Mocks

Since the business logic of a component typically creates child resources, testing the logic usually involves
mocking the implementation of child resources.  For example, if your component creates a `random.RandomPet` resource,
you can use a mock to return simulated resource state, i.e. the `RandomPet.ID` property. The actual random provider
is never called.

To configure mocking, use the `integration.WithMocks` server option and pass an implementation of `pulumi.MockResourceMonitor`.
The mock monitor receives a callback for the component resource and for each child resource as it is registered,
giving you an opportunity to return a simulated state for each child. See `integation.MockResourceMonitor` for a simple implementation.

To test a component resource, call the `Construct` method on the integration server.

```go

import (
	"testing"

	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	r "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func TestConstruct(t *testing.T) {
    myProvider, err := infer.NewProviderBuilder().
		WithComponents(
			infer.ComponentF(MyComponent),
		).
		Build()
	require.NoError(t, err)

	server, err := integration.NewServer(
		t.Context(),
		"example",
		semver.MustParse("1.0.0"),
		integration.WithProvider(myProvider),
        integration.WithMocks(&integration.MockResourceMonitor{
			NewResourceF: func(args integration.MockResourceArgs) (string, property.Map, error) {
				// NewResourceF is called as the each resource is registered
				switch {
				case args.TypeToken == "my:module:MyComponent" && args.Name == "my-component":
                default:
                    // make assertions about the component's children and return
                    // a fake id and some resource properties
				}
				return "", property.Map{}, nil
			},
		}),
	)
	require.NoError(t, err)

	// test the "my:module:MyComponent" component
	resp, err := server.Construct(p.ConstructRequest{
		Urn:    "urn:pulumi:stack::project::my:module:MyComponent::my-component",
		Inputs: property.NewMap(map[string]property.Value{
			"pi": property.New(3.14),
		}),
	})
	require.NoError(t, err)
	require.Equal(t, property.NewMap(map[string]property.Value{
		"result": property.New("foo-12345").WithSecret(true),
	}), resp.State)
}
```

## Custom Resource Lifecycle Testing
The `LifeCycleTest` struct enables testing the full lifecycle of a custom resource, including:
1. Previewing and creating resources.
2. Updating resources with new inputs.
3. Deleting resources.

It supports hooks for custom validation and assertions on resource outputs.

## Client Injection

Most providers connect to external systems using a client library of some kind. To test your provider code,
you'll need to make a suitable mock client. This section addresses how to inject a client into an infer-style provider.

The provider build syntax allows you to pre-initialize the various receivers. Use this to supply your resources, 
components, and functions with a logger, client factory, or other variables.

See `examples/configurable` for a demonstration.

### Example

```go
func TestMyResourceLifecycle(t *testing.T) {
	myProvider, err := infer.NewProviderBuilder().
		WithResources(
			infer.Resource(MyResource),
		).
		Build()
	require.NoError(t, err)

	server, err := integration.NewServer(
		t.Context(),
		"example",
		semver.MustParse("1.0.0"),
		integration.WithProvider(myProvider),
	)
	require.NoError(t, err)

	test := integration.LifeCycleTest{
		Resource: "my:module:MyResource",
		Create: integration.Operation{
			Inputs: property.Map{"key": property.New("value")},
			ExpectedOutput: &property.Map{"key": property.New("value")},
		},
	}
	test.Run(t, server)
}
```

For more details, refer to the source code and comments in the `integration.go` file, and the battery of test cases
in the `tests` package since the tests are implemented using the `integration` package.
