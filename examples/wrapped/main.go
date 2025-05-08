// Copyright 2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
