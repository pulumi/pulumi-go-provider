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

To make a server, call `integration.NewServerWithOptions` and pass the `Provider` instance to be tested.
Then, exercise the various RPC methods of your provider.

## Component Resource Mocks

Since the business logic of a component typically creates child resources, testing the logic usually involves
mocking the implementation of child resources.  For example, if your component creates a `random.RandomPet` resource,
you can use a mock to return simulated resource state, i.e. the `RandomPet.ID` property. The actual random provider
is never called.

To configure mocking, use the `integration.WithMocks` server option and pass an implementation of `pulumi.MockResourceMonitor`.
The mock monitor receives a callback for the component resource and for each child resource as it is registered,
giving you an opportunity to return a simulated state for each child. See `integation.MockMonitor` for a simple implementation.

To test a component resource, call the `Construct` method on the integration server.

## Custom Resource Lifecycle Testing
The `LifeCycleTest` struct enables testing the full lifecycle of a custom resource, including:
1. Previewing and creating resources.
2. Updating resources with new inputs.
3. Deleting resources.

It supports hooks for custom validation and assertions on resource outputs.

### Example

```go
func TestMyResourceLifecycle(t *testing.T) {
    myProvider, _ := infer.NewProviderBuilder().
		WithName("example").
		WithVersion("0.1.0").
		WithResources(
			infer.Resource(MyResource),
		).
		Build()

    server := integration.NewServerWithOptions(t.Context(), "my-provider", semver.MustParse("1.0.0"), myProvider)
    
    test := integration.LifeCycleTest{
        Resource: "my:module:MyResource",
        Create: integration.Operation{
            Inputs: property.Map{"key": property.NewStringProperty("value")},
            ExpectedOutput: &property.Map{"key": property.NewStringProperty("value")},
        },
    }
    test.Run(t, server)
}
```

For more details, refer to the source code and comments in the `integration.go` file, and the battery of test cases
in the `tests` package since the tests are implemented using the `integration` package.
