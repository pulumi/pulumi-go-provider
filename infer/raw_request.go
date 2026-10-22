// Copyright 2026, Pulumi Corporation.
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

package infer

import (
	"context"

	p "github.com/pulumi/pulumi-go-provider"
)

// RawResourceRequest is the set of untyped requests that infer decodes before calling a
// custom resource's typed methods.
type RawResourceRequest interface {
	p.CheckRequest | p.DiffRequest | p.CreateRequest | p.ReadRequest | p.UpdateRequest | p.DeleteRequest
}

type rawRequestKey[T RawResourceRequest] struct{}

func withRawRequest[T RawResourceRequest](ctx context.Context, req T) context.Context {
	return context.WithValue(ctx, rawRequestKey[T]{}, req)
}

// GetRawRequest returns the untyped request that infer is serving, such as the
// [p.CreateRequest] inside a [CustomCreate]. The second result is false when ctx doesn't
// come from a request of type T.
//
//	func (*MyResource) Create(ctx context.Context, req infer.CreateRequest[Args]) (infer.CreateResponse[State], error) {
//		raw, ok := infer.GetRawRequest[p.CreateRequest](ctx)
//		if ok && raw.Properties.Get("token").Secret() {
//			// ...
//		}
//		// ...
//	}
//
// See https://github.com/pulumi/pulumi-go-provider/issues/289.
func GetRawRequest[T RawResourceRequest](ctx context.Context) (T, bool) {
	req, ok := ctx.Value(rawRequestKey[T]{}).(T)
	return req, ok
}
