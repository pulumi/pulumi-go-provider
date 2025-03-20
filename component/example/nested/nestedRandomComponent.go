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

package nested

import (
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type NestedRandomComponent struct {
	pulumi.ResourceState
	NestedRandomComponentArgs
	Password        pulumi.StringOutput `pulumi:"password"`
	HardcodedOutput pulumi.StringOutput `pulumi:"hardcodedOutput"`
}

type NestedRandomComponentArgs struct {
	Length pulumi.IntInput `pulumi:"length"`
}

func CreateNestedRandomComponent(ctx *pulumi.Context, name string, compArgs NestedRandomComponentArgs, opts ...pulumi.ResourceOption) (*NestedRandomComponent, error) {
	comp := &NestedRandomComponent{}
	err := ctx.RegisterComponentResource("go-components:nested:NestedRandomComponent", name, comp, opts...)
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
	comp.HardcodedOutput = pulumi.String("This is a hardcoded output string from a nested module.").ToStringOutput()

	return comp, nil
}
