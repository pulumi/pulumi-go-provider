// Copyright 2024, Pulumi Corporation.
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

package annotate

// The methods of Annotator must be called on pointers to fields of their receivers, or on
// their receiver itself.
//
//	func (*s Struct) Annotated(a Annotator) {
//		a.Describe(&s, "A struct")            // Legal
//		a.Describe(&s.field1, "A field")      // Legal
//		a.Describe(s.field2, "A field")       // Not legal, since the pointer is missing.
//		otherS := &Struct{}
//		a.Describe(&otherS.field1, "A field") // Not legal, since describe is not called on its receiver.
//	}
type Annotator interface {
	// Annotate a struct field with a text description.
	Describe(i any, description string)

	// Annotate a struct field with a default value. The default value must be a primitive
	// type in the pulumi type system.
	SetDefault(i any, defaultValue any, env ...string)

	// Set the token of the annotated type.
	//
	// module and name should be valid Pulumi token segments. The package name will be
	// inferred from the provider.
	//
	// For example:
	//
	//	a.SetToken("mymodule", "MyResource")
	//
	// On a provider created with the name "mypkg" will have the token:
	//
	//	mypkg:mymodule:MyResource
	//
	SetToken(module, name string)
}

// Annotated is used to describe the fields of an object or a resource. Annotated can be
// implemented by `CustomResource`s, the input and output types for all resources and
// invokes, as well as other structs used the above.
type Annotated interface {
	Annotate(Annotator)
}
