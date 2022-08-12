# Examples

Each example folder contains:

1. A complete and functional package, buildable with `go build ./...`.
2. A `consumer/Pulumi.yaml` which will exercise the provider.

You can build the provider and then run the `consumer/Pulumi.yaml` example against it with the folling 1-liner:

```sh
go build github.com/pulumi/pulumi-go-provider/examples/$(basename $PWD) && pulumi plugin install resource $(basename $PWD) v0.1.0 -f $(basename $PWD) --reinstall && (cd consumer && pulumi up)
```
