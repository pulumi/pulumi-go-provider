# Middleware

## Layers
The `middleware` package contains various middleware layers with which to compose a provider.

| Package         | Description                                                                                     |
|-----------------|-------------------------------------------------------------------------------------------------|
| `cancel`        | Provides middleware to tie Pulumi's cancellation system to Go `context.Context` cancellation. |
| `complexconfig` | Adds middleware for schema-informed complex configuration encoding/decoding.                    |
| `context`       | Allows systemic wrapping of `context.Context` before invoking a subsidiary provider.            |
| `dispatch`      | Dispatches calls by type token to resource-level abstractions.                                  |
| `rpc`           | Wraps a legacy provider (`rpc.ResourceProviderServer`) into a `Provider`.                       |
| `schema`        | Generates Pulumi schema based on resource and function abstractions.                            |

## Types

The layers operate on a set of common types defined by this package.

| Type               | Description                                                                 |
|--------------------|-----------------------------------------------------------------------------|
| `CustomResource`   | Defines a high-level interface for Pulumi custom resources.                |
| `ComponentResource`| Defines an interface for Pulumi component resources.                      |
| `Invoke`           | Defines an interface for Pulumi functions (invokes).                      |

