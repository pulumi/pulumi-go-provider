# Examples

Each example folder contains:

1. A complete and functional package, buildable with `go build ./...`.
2. A `consumer/Pulumi.yaml` which will exercise the provider.

You can build the provider and then run the `consumer/Pulumi.yaml` example against it with the folling 1-liner:

```sh
go build ./... && NAME="$(basename ${PWD%/*})" pulumi plugin install resource ${NAME} v0.1.0 -f ${NAME} --reinstall && cd consumer && pulumi up && cd ..
```
