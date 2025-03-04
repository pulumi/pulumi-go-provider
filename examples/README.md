# Examples

## What's in an example

Each example folder contains:

1. A complete and functional Pulumi provider.
2. A `consumer/Pulumi.yaml` which will exercise the provider.

You can build the provider and then run the `consumer/Pulumi.yaml` example against it using `make`:

```sh
make examples/<example name>/test
```

To run the `echo` example, you would run `make examples/echo/test`.

## Current examples

- [assets](./assets/main.go) provides an example for returning [Pulumi assets](https://www.pulumi.com/docs/iac/concepts/assets-archives/) in [`infer`][infer] based providers.
- [auto-naming](./auto-naming/main.go) shows an example of naive [auto-naming](https://www.pulumi.com/docs/iac/concepts/resources/names/#autonaming) for an [`infer`][infer] based resource.
- [credentials](./credentials/main.go) shows using [`infer`][infer] to define, accept and return an enum. It also
  has a custom `Configure` workflow.
- [dna-store](./dna-store/main.go) shows using [`infer`][infer] to implement `Read`.
- [echo](./echo/main.go) shows how to use [`github/pulumi/pulumi-go-provider`](../README.md) without any
  middleware. It defines a simple custom resource.
- [file](./file/main.go) uses [`infer`][infer] to define a resource that manages a file. It shows a custom
  `Check`, `Diff` and `Update` implementation.
- [random-login](./random-login/main.go) shows using [`infer`][infer] to define both component and custom
  resources.
- [str](./str/main.go) shows using [`infer`][infer] to define functions on strings.

[infer]: ../infer/README.md
