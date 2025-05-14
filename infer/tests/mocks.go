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

package tests

import (
	context "context"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type TestFunction[I, O any] interface {
	infer.Annotated
	infer.Fn[I, O]
}

type TestConfig interface {
	infer.CustomConfigure

	// TODO: We can't use infer.CustomCheck here because we expect the config
	// type C to always be its own inputs, which prevents us from providing a
	// mock config object that implements Check[C]. Something similar to
	// Resource would be more ergonomic (a type R which can operate on a
	// different input type I).
	Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[*MockTestConfig], error)
}

type TestResource[I, O any] interface {
	infer.Annotated
	infer.CustomCheck[I]
	infer.CustomCreate[I, O]
	infer.CustomDelete[O]
	infer.CustomDiff[I, O]
	infer.CustomRead[I, O]
	infer.CustomUpdate[I, O]
}

type TestComponent[I any, O pulumi.ComponentResource] interface {
	infer.Annotated
	infer.ComponentResource[I, O]
}

type TestHost interface {
	pgp.Host
}
