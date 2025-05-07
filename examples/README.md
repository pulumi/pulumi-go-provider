# Examples

This repo provides full end-to-end examples for the `pulumi-go-provider` library. Each example folder contains:

1. A complete and functional Pulumi provider.
2. An YAML-based example program (in `consumer/Pulumi.yaml`) to exercise the provider.

You can build the provider and then run the `consumer/Pulumi.yaml` example against it using `make`:

```sh
make examples/<example name>/test
```

To run the `echo` example, you would run `make examples/echo/test`.

## Infer-Based Examples

- [assets](./assets/main.go) provides an example for returning [Pulumi assets](https://www.pulumi.com/docs/iac/concepts/assets-archives/) in [`infer`][infer] based providers.
- [component-program](./component-program/main.go) shows an example of how to use [`infer`][infer] to create a simple provider using a [Pulumi component resource program](https://www.pulumi.com/docs/iac/concepts/resources/components/#authoring-a-new-component-resource).
- [auto-naming](./auto-naming/main.go) shows an example of naive [auto-naming](https://www.pulumi.com/docs/iac/concepts/resources/names/#autonaming) for an [`infer`][infer] based resource.
- [credentials](./credentials/main.go) shows using [`infer`][infer] to define, accept and return an enum. It also
  has a custom `Configure` workflow.
- [dna-store](./dna-store/main.go) shows using [`infer`][infer] to implement `Read`.
- [file](./file/main.go) uses [`infer`][infer] to define a resource that manages a file. It shows a custom
  `Check`, `Diff` and `Update` implementation.
- [random-login](./random-login/main.go) shows using [`infer`][infer] to define both component and custom
  resources. It also shows using resources as arguments and how to nest components using a generated SDK.
- [str](./str/main.go) shows using [`infer`][infer] to define functions on strings.

[infer]: ../infer/README.md

## Low-Level Provider Examples

- [echo](./echo/main.go) shows how to use [`github/pulumi/pulumi-go-provider`](../README.md) without any
  middleware. It defines a simple custom resource.
