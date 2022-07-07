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

import (
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type ID = string

// A Custom resource which can manage its own CRUD lifecycle.
//
// Custom facilitates the only non-optional CRUD operations: Create and Delete.
//
// To support the rest of the Pulumi Resource life cyle, the behavior of Custom resources
// can be augmented by implementing other interfaces:
// - Update
// - Diff
// - Check
// - Read
//
// When Update is not implemented, the Pulumi Engine will error if an update is requested.
//
// When Diff is not implemented, it will perform a recursive diff of each input field of
// the struct. When Update is not implemented, Diff will always force a replace if there
// any changes. If you override Diff so that the resource is requested to update without
// also overriding Update, the Pulumi Engine will error.
//
// When Check if not implemented, it will blindly accept any input that correctly
// deserializes to the given resource.
//
// When Read is not implemented, the Pulumi Engine will error when read is called.

// The base interface for a custom resource.
type Custom interface {
	// Create a Custom resource and return its ID.
	//
	// Resource input properties will be applied to the custom resource before any methods
	// are called. Create should set any output properties it wants to export.
	//
	// The context passed to Create may be canceled. Create should clean up and return as
	// soon as possible.
	//
	// This means that implementing this method correctly requires passing the Resource
	// implementer by reference.
	//
	// Warning: Mutating the receiver asynchronously after Create has returned may lead to
	// invalid behavior.
	Create(ctx Context, name string, preview bool) (ID, error)

	// Delete the Custom resource.
	//
	// Resource properties will be applied to the custom resource before the Delete
	// method is called.
	Delete(ctx Context, id ID) error
}

// A Custom resource which knows how to handle changing inputs.
type Update interface {
	Custom

	// Update the resource without requiring a replace operation.
	//
	// The resource is its old input and output values. `new` is the same type as the
	// concrete implementer of Update and holds the new inputs.
	Update(ctx Context, id ID, new interface{}, ignoreChanges []string, preview bool) error
}

// A Custom resource which knows how to compare against another instance of itself.
type Diff interface {
	Custom

	// Diff the resource against another resource of the same type. `new` has the same
	// type as the implementer of Diff. Diff should ignore changes to any field specified
	// by `ignoreChanges`.
	Diff(ctx Context, id ID, new interface{}, ignoreChanges []string) (*pulumirpc.DiffResponse, error)
}

// A Custom resource which knows how to validate itself.
type Check interface {
	Custom

	// Check validates that the given property bag is valid for a resource of the given type and returns the inputs
	// that should be passed to successive calls to Diff, Create, or Update for this resource. As a rule, the provider
	// inputs returned by a call to Check should preserve the original representation of the properties as present in
	// the program inputs. Though this rule is not required for correctness, violations thereof can negatively impact
	// the end-user experience, as the provider inputs are using for detecting and rendering diffs.
	Check(ctx Context, news interface{}, sequenceNumber int) ([]CheckFailure, error)
}

type CheckFailure struct {
	Property string // the property that failed validation.
	Reason   string // the reason that the property failed validation.
}

type Read interface {
	Custom
	Read(ctx Context, id string) (error)
}

type Annotator interface {
	// Annotate a a struct field with a text description.
	Describe(i any, description string)

	// Annotate a a struct field with a default value. The default value must be a primitive
	// type in the pulumi type system.
	SetDefault(i any, defaultValue any)
}

type Annotated interface {
	Annotate(Annotator)
}
