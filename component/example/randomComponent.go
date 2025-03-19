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

package main

import (
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type RandomComponent struct {
	pulumi.ResourceState
	RandomComponentArgs
	Password pulumi.StringOutput `pulumi:"password"`
}

type RandomComponentArgs struct {
	Length pulumi.IntInput `pulumi:"length"`
}

func NewMyComponent(ctx *pulumi.Context, name string, compArgs RandomComponentArgs, opts ...pulumi.ResourceOption) (*RandomComponent, error) {
	comp := &RandomComponent{}
	err := ctx.RegisterComponentResource("pkg:index:MyRandomComponent", name, comp, opts...)
	if err != nil {
		return nil, err
	}

	pArgs := &random.RandomPasswordArgs{
		Length: compArgs.Length,
	}

	password, err := random.NewRandomPassword(ctx, name+"-password", pArgs, pulumi.Parent(comp))
	if err != nil {
		return nil, err
	}

	comp.Password = password.Result

	return comp, nil
}
