// Copyright 2022, Pulumi Corporation.
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

package types

import (
	"context"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// GenericInput is an input type that accepts Generic and GenericOutput values.
type GenericInput interface {
	pulumi.Input

	ToGenericOutput() GenericOutput
	ToGenericOutputWithContext(ctx context.Context) GenericOutput

	ToGenericPtrOutput() GenericPtrOutput
	ToGenericPtrOutputWithContext(ctx context.Context) GenericPtrOutput
}

// Generic is an input type for int values.
type Generic struct {
	item any
}

// ElementType returns the element type of this Input (int).
func (in Generic) ElementType() reflect.Type {
	return reflect.TypeOf(in.item)
}

func (in Generic) ToGenericOutput() GenericOutput {
	return pulumi.ToOutput(in).(GenericOutput)
}

func (in Generic) ToGenericOutputWithContext(ctx context.Context) GenericOutput {
	return pulumi.ToOutputWithContext(ctx, in).(GenericOutput)
}

func (in Generic) ToGenericPtrOutput() GenericOutput {
	return in.ToGenericPtrOutputWithContext(context.Background())
}

func (in Generic) ToGenericPtrOutputWithContext(ctx context.Context) GenericOutput {
	return in.ToGenericOutputWithContext(ctx).ToGenericPtrOutputWithContext(ctx)
}

type GenericOutput struct{ *pulumi.OutputState }

func (o GenericOutput) ElementType() reflect.Type {
	return o.
}

func (o GenericOutput) ToGenericOutput() GenericOutput {
	return o
}

func (o GenericOutput) ToGenericOutputWithContext(ctx context.Context) GenericOutput {
	return o
}

func (o GenericOutput) ToGenericPtrOutput() GenericOutput {
	return o.ToGenericPtrOutputWithContext(context.Background())
}

func (o GenericOutput) ToGenericPtrOutputWithContext(ctx context.Context) GenericOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v Generic) Generic {
		return Generic{
			item: &v.item,
		}
	}).(GenericOutput)
}
