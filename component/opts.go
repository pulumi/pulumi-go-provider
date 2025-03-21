// Copyright 2016-2025, Pulumi Corporation.
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

package component

// providerOpts represents options for configuring the provider.
type providerOpts struct {
	// Name of the provider
	name string
	// version of the provider
	version string
	// components is a list of components that are inferred from the code.
	components map[Resource]struct{}
}

// providerOpt is a function that modifies ProviderOpts.
type providerOpt func(*providerOpts)

// WithResource adds a component resource to the provider options to be served by the provider.
func WithResources(r ...Resource) providerOpt {
	return func(opts *providerOpts) {
		if opts.components == nil {
			opts.components = make(map[Resource]struct{})
		}
		for _, resource := range r {
			opts.components[resource] = struct{}{}
		}
	}
}

// WithName sets the provider name.
func WithName(name string) providerOpt {
	return func(opts *providerOpts) {
		opts.name = name
	}
}

// WithVersion sets the provider version.
func WithVersion(version string) providerOpt {
	return func(opts *providerOpts) {
		opts.version = version
	}
}
