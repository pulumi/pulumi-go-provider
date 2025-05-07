# Middleware

Middleware are composable layers that extend or modify the behavior of a `Provider` by implementing
one or more of its functions. Each middleware layer operates on a set of common types and interfaces defined by this package.

Middleware enables features such as schema generation, request dispatching, and request cancellation.

For example, the `schema` middleware implements `Provider.GetSchema` based on options
provided to `schema.Wrap`.

## Layers

The `middleware` package contains various middleware layers with which to compose a provider.

| Package         | Description                                                                                     |
|-----------------|-------------------------------------------------------------------------------------------------|
| `cancel`        | Provides middleware to tie Pulumi's cancellation system to Go `context.Context` cancellation.   |
| `complexconfig` | (deprecated) Adds middleware for schema-informed complex configuration encoding/decoding.       |
| `context`       | Allows systemic wrapping of `context.Context` before invoking a subsidiary provider.            |
| `dispatch`      | Dispatches calls by type token to resource-level abstractions.                                  |
| `rpc`           | Wraps a legacy provider (`rpc.ResourceProviderServer`) into a `Provider`.                       |
| `schema`        | Generates Pulumi schema based on resource and function abstractions.                            |

## Types

The layers operate on a set of common types defined by this package.

| Type               | Description                                                                 |
|--------------------|-----------------------------------------------------------------------------|
| `CustomResource`   | Defines a high-level interface for Pulumi custom resources.                 |
| `ComponentResource`| Defines an interface for Pulumi component resources.                        |
| `Invoke`           | Defines an interface for Pulumi functions (invokes).                        |

