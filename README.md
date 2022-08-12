# pulumi-go-provider

A framework for building Go Providers for Pulumi

_Note_: This library is in active development, and not everthing is hooked up. You should
expect breaking changes as we fine tune the exposed APIs. We definitely appreciate
community feedback, but you should probably wait to port any existing providers over.

## The "Hello, Pulumi" Provider

Here we provide the code to create an entire native provider consumable from any of the
Pulumi languages (TypeScript, Python, Go, C#, Java and Pulumi YAML). The binary generated
from this file has the capability to serve the provider as well as run schema generation.

```go
package main

import (
	"fmt"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	r "github.com/pulumi/pulumi-go-provider/resource"
)

func main() {
	// Here we list the facilities that the provider will support.
	p.Run("hello", semver.Version{Minor: 1},
		// This provider only supports a single custom resource, so this section is brief.
		p.Resources(&HelloWorld{}))
}

// This is how we define a custom resource.
type HelloWorld struct {
	// Fields that have a `pulumi` tag are used by the provider framework. Fields marked
	// `optional` are optional, so they should have a pointer ahead of their type.
	Name *string `pulumi:"name,optional"`
	// Fields with `provider:"output" are outputs in the Pulumi type system, otherwise
	// they are inputs.`
	Phrase string `pulumi:"phrase" provider:"output"`
}

// The r.Custom interface is what defines the base of a custom resource. It has 2 methods,
// Create and Delete. Create is called to create a new resource.
func (r *HelloWorld) Create(ctx r.Context, name string, preview bool) (r.ID, error) {
	// We indicate we will not know the output of r.Phrase during preview.
	ctx.MarkComputed(&r.Phrase)
	n := name
	// Inputs are already on the struct when Create is called.
	if r.Name != nil {
		n = *r.Name
	}

	// And we add outputs by assigning them to the their fields before we return.
	r.Phrase = fmt.Sprintf("Hello, %s!", n)

	// name is the name component of the resource URN. We pass that as the ID here, but we
	// should append randomness to it later.
	return name, nil
}

// We don't need to do anything to delete this resource, but this is where we would do it.
func (r *HelloWorld) Delete(ctx r.Context, id r.ID) error {
	return nil
}

```

The framework is doing a lot of work for us here. Since we didn't implement `Diff` it is
assumed to be structural. The diff will require a replace if any field changes, since we
didn't implement update. `Check` will always pass and `Read` will always fail as
unimplemented. All of these methods are available as interfaces in the `resource` module,
and implementing them will replace the default behavior.

## Translating Go into the Pulumi type system

The goal of the library is to allow you to write idiomatic Go as much as possible. To that
end, your Go code is translated into the Pulumi type system automatically. Here are some
useful rules:

- Tokens are generated directly from type names. A resource defined with the struct `Bar`
  in the `foo` module will get assigned the token `${pkg}:foo:Bar`.
- Custom resources don't interact with Pulumi Input/Output types. They should have raw types and must
  manually manage dealing with `nil` values.
- Component resources do deal with Pulumi Input/Output types. They should only have raw
  fields when the types are `plain`.

### Struct tags

This library determines how to serialize Go types via struct tags. It uses 2 namespaces:
`pulumi` and `provider`. Under the `pulumi` namespace, we support the following tags:

- `name` - Name is the fist tag in the after `pulumi:`. It is required for any field used by
  the provider. For example `pulumi:"foo"` tags a field with the name `foo`.
- `optional` - Marking a field as optional in the Pulumi type system. To mark `foo` as an
  optional field, we would write: `pulumi:"foo,optional"`.

The rest of the supported tags are under the `provider` namespace:

- `output` - Marks a field as an output instead of an input.
- `secret` - Marks a field as secret. This will effect schema generation, but not the
  internal behavior of the resource.
- `replaceOnChanges` - Marks the resource as needing a replace whenever the field changes.
