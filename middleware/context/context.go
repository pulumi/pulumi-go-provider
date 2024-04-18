// Copyright 2022-2024, Pulumi Corporation.
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

// Context allows systemic wrapping of provider.Context before invoking a subsidiary provider.
package context

import (
	p "github.com/pulumi/pulumi-go-provider"
)

// The function applied to each provider.Context that passes through this Provider.
type Wrapper = func(p.Context) p.Context

// Create a Provider that calls `wrapper` on each context passed into `provider`.
func Wrap(provider p.Provider, wrapper Wrapper) p.Provider {
	return p.Provider{
		GetSchema:   delegateIO(wrapper, provider.GetSchema),
		Cancel:      delegate(wrapper, provider.Cancel),
		CheckConfig: delegateIO(wrapper, provider.CheckConfig),
		DiffConfig:  delegateIO(wrapper, provider.DiffConfig),
		Configure:   delegateI(wrapper, provider.Configure),
		Invoke:      delegateIO(wrapper, provider.Invoke),
		Check:       delegateIO(wrapper, provider.Check),
		Diff:        delegateIO(wrapper, provider.Diff),
		Create:      delegateIO(wrapper, provider.Create),
		Read:        delegateIO(wrapper, provider.Read),
		Update:      delegateIO(wrapper, provider.Update),
		Delete:      delegateI(wrapper, provider.Delete),
		Construct:   delegateIO(wrapper, provider.Construct),
	}
}

func delegateIO[I, O any, F func(p.Context, I) (O, error)](wrapper Wrapper, method F) F {
	if method == nil {
		return nil
	}
	return func(ctx p.Context, req I) (O, error) { return method(wrapper(ctx), req) }
}

func delegateI[I any, F func(p.Context, I) error](wrapper Wrapper, method F) F {
	if method == nil {
		return nil
	}
	return func(ctx p.Context, req I) error { return method(wrapper(ctx), req) }
}

func delegate[F func(p.Context) error](wrapper Wrapper, method F) F {
	if method == nil {
		return nil
	}
	return func(ctx p.Context) error { return method(wrapper(ctx)) }
}
