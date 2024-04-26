# Examples

Each example folder contains:

1. A complete and functional package, buildable with `go build ./...`.
2. A `consumer/Pulumi.yaml` which will exercise the provider.

You can build the provider and then run the `consumer/Pulumi.yaml` example against it with the following 1-liner:

```sh
name=$(basename $PWD) && go build -o "pulumi-resource-$name" github.com/pulumi/pulumi-go-provider/examples/$name && pulumi plugin install resource $name v0.1.0 -f "pulumi-resource-$name" --reinstall && (cd consumer && pulumi up)
```
