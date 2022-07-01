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

package resource

import "github.com/pulumi/pulumi/sdk/v3/go/pulumi"

// A Component is a component resource which understands how to construct itself.
type Component interface {
	pulumi.ComponentResource

	// Construct the Component.
	//
	// When construct is called, it will already have it's input fields assigned to their
	// appropriate values. It should create its child resources, making sure to parent
	// them to itself. It should assign to any value that it outputs.
	//
	// For example:
	// ```go
	// type RandomLogin struct {
	//     pulumi.ResourceState
	//
	//     // Outputs
	//     Username pulumi.StringOutput `pulumi:"username"`
	//     Password pulumi.StringOutput `pulumi:"password"`
	//
	//     // Inputs
	//     PasswordLength pulumi.IntPtrInput `pulumi:"passwordLength"`
	// }
	//
	// func (r *RandomLogin) Construct(name string, ctx *pulumi.Context) error {
	// 	   pet, err := random.NewRandomPet(ctx, name+"-pet", &random.RandomPetArgs{}, pulumi.Parent(r))
	// 	   if err != nil {
	// 	       return err
	// 	   }
	// 	   r.Username = pet.ID().ToStringOutput()
	// 	   var length pulumi.IntInput = pulumi.Int(16)
	// 	   if r.PasswordLength != nil {
	// 	       length = r.PasswordLength.ToIntPtrOutput().Elem()
	// 	   }
	// 	   password, err := random.NewRandomPassword(ctx, name+"-password", &random.RandomPasswordArgs{
	// 	       Length: length,
	// 	   }, pulumi.Parent(r))
	// 	   if err != nil {
	// 	   	   return err
	// 	   }
	// 	   r.Password = password.Result.ToStringOutput()
	// 	   return nil
	// }
	// ```
	//
	// You will observe that there is *no* need to call RegisterComponentResource or
	// RegisterResourceOutputs.
	Construct(name string, ctx *pulumi.Context) error
}
