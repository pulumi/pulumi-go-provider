// This example shows using [github.com/pulumi/pulumi-go-provider] to wrap and extend an existing
// provider that was written using the Pulumi Go SDK.
package main

import (
	"context"
	_ "embed"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	wrapcancel "github.com/pulumi/pulumi-go-provider/middleware/cancel"
	wraprpc "github.com/pulumi/pulumi-go-provider/middleware/rpc"

	pprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
)

//go:embed schema-legacy.json
var pulumiSchema []byte

func main() {
	err := p.RunProviderF(context.Background(), "xyz", "v0.1.0-dev", func(host *pprovider.HostClient) (p.Provider, error) {
		// create a legacy provider of type pulumirpc.ResourceProviderServer
		legacy, err := makeProvider(host, "xyz", "v0.1.0-dev", pulumiSchema)
		if err != nil {
			return p.Provider{}, err
		}

		// wrap the legacy provider into a Provider
		p := wraprpc.Provider(legacy)

		// apply middleware to the wrapped provider such as cancelation support,
		// which is not supported directly by the legacy provider (see xyxProvider.Cancel).
		p = wrapcancel.Wrap(p)

		return p, err
	})

	if err != nil {
		os.Exit(1)
	}
}
