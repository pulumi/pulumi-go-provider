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

// GenericInput is an input type that accepts Generic and Output values.
type GenericInput[T any] interface {
	pulumi.Input

	ToOutput() Output[T]
	ToOutputWithContext(ctx context.Context) Output[T]

	ToPtrOutput() Output[*T]
	ToPtrOutputWithContext(ctx context.Context) Output[*T]

	GetNilType() T
}

// Generic is an input type for int values.
type Input[T any] struct {
	item T
}

// ElementType returns the element type of this Input (int).
func (in Input[T]) ElementType() reflect.Type {
	return reflect.TypeOf(in.item)
}

func (in Input[T]) ToOutput() Output[T] {
	return pulumi.ToOutput(in).(Output[T])
}

func (in Input[T]) ToOutputWithContext(ctx context.Context) Output[T] {
	return pulumi.ToOutputWithContext(ctx, in).(Output[T])
}

func (in Input[T]) GetNilType() T {
	return *(*T)(nil) //Hacky but needed for reflection
}

/*
func (in Input[T]) ToPtrOutput() Output[*T] {
	return in.ToPtrOutputWithContext(context.Background())
}

func (in Input[T]) ToPtrOutputWithContext(ctx context.Context) Output[*T] {
	return in.ToOutputWithContext(ctx).ToPtrOutputWithContext(ctx)
}*/

type Output[T any] struct{ *pulumi.OutputState }

func (o Output[T]) ElementType() reflect.Type {
	//return o.
	return reflect.TypeOf((*T)(nil)).Elem()
}

func (o Output[T]) ToOutput() Output[T] {
	return o
}

func (o Output[T]) ToOutputWithContext(ctx context.Context) Output[T] {
	return o
}

/*
func (o Output[T]) ToPtrOutput() Output[*T] {
	return o.ToPtrOutputWithContext(context.Background())
}

func (o Output[T]) ToPtrOutputWithContext(ctx context.Context) Output[*T] {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v T) *T {
		return &v
	}).(Output[*T])
}*/
