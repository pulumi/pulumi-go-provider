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

package provider

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/pulumi/pulumi-go-provider/internal/key"
)

// PluginIdentity describes the plugin serving the current request: the package it
// implements, the version it was built as, and the URL the engine can download it from.
//
// A component that registers a resource of its own package from inside Construct needs
// this. Such a registration goes straight to [pulumi.Context.RegisterResource], which
// applies no package defaults — unlike a registration made through a generated SDK,
// where codegen bakes the version and the download URL into the resource's default
// options. Without them the engine records a provider with neither field, and resolves
// the plugin later from the conventional github.com/pulumi/pulumi-<name> location. For a
// third-party provider no such repository exists, so any later operation that starts
// with a cold plugin cache — a destroy from a fresh workspace is the usual one — fails to
// load the plugin at all.
//
// [ConstructRequest] cannot carry these: the Construct gRPC request has no field for
// either, so the values come from the running provider itself.
type PluginIdentity struct {
	// Package is the Pulumi package this provider serves, e.g. "random".
	Package string

	// Version is the provider's own version, e.g. "1.2.3". Empty if the provider was
	// run without one.
	Version string

	// DownloadURL is the URL the engine should download this provider's plugin from,
	// e.g. "github://api.github.com/pulumi/pulumi-random". Empty unless the provider
	// set one, which [github.com/pulumi/pulumi-go-provider/infer.ProviderBuilder.WithPluginDownloadURL]
	// does.
	DownloadURL string
}

// ResourceOptions returns the options that record this identity on a resource.
//
// Apply them only to resources of this provider's own [PluginIdentity.Package]. They name
// one specific plugin, so passing them to a resource of another package — an aws:s3/bucket
// created alongside your own — sends the engine looking for that package's plugin at your
// version and your URL, which is not what you want. Give each such resource the options
// belonging to its own package instead, or none at all.
//
// Empty fields are skipped, so a provider that set no download URL still gets its version
// recorded.
func (id PluginIdentity) ResourceOptions() []pulumi.ResourceOption {
	opts := make([]pulumi.ResourceOption, 0, 2)
	if id.Version != "" {
		opts = append(opts, pulumi.Version(id.Version))
	}
	if id.DownloadURL != "" {
		opts = append(opts, pulumi.PluginDownloadURL(id.DownloadURL))
	}
	return opts
}

// GetPluginIdentity reports the identity of the plugin serving the current request.
//
// The [context.Context] is the one passed to a [Provider] method. Inside a component's
// Construct, where the Pulumi Go SDK hands over a [pulumi.Context] instead, reach it with
// [pulumi.Context.Context]:
//
//	func (c *Component) Construct(
//		ctx *pulumi.Context, name, typ string, args Args, opts pulumi.ResourceOption,
//	) (*Component, error) {
//		self := provider.GetPluginIdentity(ctx.Context())
//
//		child := &MyResourceState{}
//		err := ctx.RegisterResource("my-pkg:index:Child", name+"-child", args, child,
//			append(self.ResourceOptions(), pulumi.Parent(comp))...)
//		...
//	}
//
// Fields the provider never supplied come back empty; the call itself never fails.
func GetPluginIdentity(ctx context.Context) PluginIdentity {
	var id PluginIdentity
	if info, ok := ctx.Value(key.RuntimeInfo).(RunInfo); ok {
		id.Package = info.PackageName
		id.Version = info.Version
	}
	if url, ok := ctx.Value(key.PluginDownloadURL).(string); ok {
		id.DownloadURL = url
	}
	return id
}
