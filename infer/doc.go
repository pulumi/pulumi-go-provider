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

// The `infer` module provides infrastructure to infer Pulumi component resources, custom
// resources and functions from go code.
//
// ## Defining a component resource
//
// Here we will define a component resource that aggregates two custom resources from the
// random provider. Our component resource will serve a username, derived from either
// random.RandomId or random.RandomPet. It will also serve a password, derived from
// random.RandomPassword. We will call the component resource `Login`.
//
// To encapsulate the idea of a new component resource, we define the resource, its inputs
// and its outputs:
//
// ```go
// type Login struct{}
// type LoginArgs struct {
//   PasswordLength pulumi.IntPtrInput `pulumi:"passwordLength"`
//   PetName        bool               `pulumi:"petName"`
// }
//
// type LoginState struct {
//   pulumi.ResourceState
//
// 	 PasswordLength pulumi.IntPtrInput `pulumi:"passwordLength"`
// 	 PetName        bool               `pulumi:"petName"`
// 	 // Outputs
// 	 Username pulumi.StringOutput `pulumi:"username"`
// 	 Password pulumi.StringOutput `pulumi:"password"`
// }
// ```
//
// Each field is tagged with `pulumi:"name"`. Pulumi (and the infer module) only acts on
// fields with this tag. Pulumi names don't need to match up with with field names, but
// the should be lowerCamelCase. Fields also need to be exported (capitalized) to interact
// with Pulumi.
//
// Most fields are Input or Output types, which means they accept pulumi inputs and
// outputs. We will make a decision based on `PetName`, so it is simply a `bool`.
//
// Now that we have defined the type of the component, we need to define how to actually
// construct the component resource:
//
// ```go
// func (r *Login) Construct(ctx *pulumi.Context, name, typ string, args LoginArgs, opts pulumi.ResourceOption) (*LoginState, error) {
// 	comp := &LoginState{}
// 	err := ctx.RegisterComponentResource(typ, name, comp, opts)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if args.PetName {
// 		pet, err := random.NewRandomPet(ctx, name+"-pet", &random.RandomPetArgs{}, pulumi.Parent(comp))
// 		if err != nil {
// 			return nil, err
// 		}
// 		comp.Username = pet.ID().ToStringOutput()
// 	} else {
// 		id, err := random.NewRandomId(ctx, name+"-id", &random.RandomIdArgs{
// 			ByteLength: pulumi.Int(8),
// 		}, pulumi.Parent(comp))
// 		if err != nil {
// 			return nil, err
// 		}
// 		comp.Username = id.ID().ToStringOutput()
// 	}
// 	var length pulumi.IntInput = pulumi.Int(16)
// 	if args.PasswordLength != nil {
// 		length = args.PasswordLength.ToIntPtrOutput().Elem()
// 	}
// 	password, err := random.NewRandomPassword(ctx, name+"-password", &random.RandomPasswordArgs{
// 		Length: length,
// 	}, pulumi.Parent(comp))
// 	if err != nil {
// 		return nil, err
// 	}
// 	comp.Password = password.Result
//
// 	return comp, nil
// }
// ```
//
// This works exactly like defining a component resource in Pulumi Go normally does with 1
// exception. It is not necessary to call `ctx.RegisterComponentResourceOutputs`.
//
// The last step in defining the component is serving it. Here we define the provider,
// telling it that it should the `Login` component. We then run that provider in `main`.
//
// ```go
// func main() {
// 	err := p.RunProvider("", semver.Version{Minor: 1}, provider())
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
// 		os.Exit(1)
//   }
// }
//
// func provider() p.Provider {
// 	return infer.NewProvider().
// 		WithComponents(infer.Component[*Login, LoginArgs, *LoginState]())
// }
// ```
//
// This is all it takes to serve a component provider.
//
// ## Defining a custom resource
package infer
