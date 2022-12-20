package openapi

import (
	"fmt"
	"net/http"
	"path"
	"reflect"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/hashicorp/go-multierror"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	res "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	p "github.com/pulumi/pulumi-go-provider"
	s "github.com/pulumi/pulumi-go-provider/middleware/schema"
)

type Operation struct {
	openapi3.Operation

	Path   string
	Client *http.Client
}

type properties struct {
	props    map[string]schema.PropertySpec
	required codegen.StringSet
}

func (p *properties) addProp(name string, prop schema.PropertySpec) {
	if p.props == nil {
		p.props = map[string]schema.PropertySpec{}
	}
	p.props[name] = prop
}

func (p *properties) unionWith(other properties) error {
	if other.props == nil {
		return nil
	}
	if p.props == nil {
		p.props = map[string]schema.PropertySpec{}
	}
	if p.required == nil {
		p.required = codegen.NewStringSet()
	}
	p.required = p.required.Union(other.required)

	var errs multierror.Error
	for name, prop := range other.props {
		existing, ok := p.props[name]
		if !ok {
			p.props[name] = prop
			continue
		}

		// A much easier version of type equality can be tolerated here.
		if !reflect.DeepEqual(existing.TypeSpec, prop.TypeSpec) {
			errs.Errors = append(errs.Errors,
				fmt.Errorf("type mismatch on property %#v: %#v != %#v",
					name, existing.TypeSpec, prop.TypeSpec))
		}
	}
	return errs.ErrorOrNil()
}

func (op *Operation) run(ctx p.Context, inputs res.PropertyMap) (res.PropertyMap, error) {
	panic("unimplemented")
}

func (op *Operation) schemaInputs(reg s.RegisterDerivativeType) (properties, error) {
	props := &properties{
		props:    map[string]schema.PropertySpec{},
		required: codegen.NewStringSet(),
	}
	r := registerTypes{
		path: op.Path,
		reg:  reg,
	}

	for _, paramRef := range op.Parameters {
		param := paramRef.Value
		spec, required := r.extendPath(param.Name).propFromSchema(param.Schema)
		if param.Required || required {
			props.required.Add(param.Name)
		}

		if param.Deprecated {
			spec.DeprecationMessage = "This resource is depreciated."
		}
		if param.Description != "" {
			spec.Description = param.Description
		}
		props.props[param.Name] = spec
	}

	var err error
	if body := op.Operation.RequestBody; body != nil {
		err = r.extractTypes(reg, props, body.Value.Content)
	}
	return *props, err
}

func (r registerTypes) refFromSchema(ref string) schema.TypeSpec {
	panic("unimplemented")
}

type registerTypes struct {
	path string
	reg  s.RegisterDerivativeType
}

func (r registerTypes) extendPath(segment string) registerTypes {
	r.path += "/" + segment
	return r
}

func (r registerTypes) typeFromSchema(typ *openapi3.SchemaRef) schema.TypeSpec {
	if typ == nil {
		return schema.TypeSpec{Ref: "pulumi.json#/Any"}
	}

	if typ.Ref != "" {
		return r.refFromSchema(typ.Ref)
	}

	// We have a value, so we try to infer the type
	v := typ.Value

	// Look for primitives
	switch v.Type {
	case "string", "number", "integer", "boolean":
		return schema.TypeSpec{Type: v.Type}

	case "object":
		element := r.typeFromSchema(v.AdditionalProperties)
		return schema.TypeSpec{
			Type:                 v.Type,
			AdditionalProperties: &element,
		}
	case "array":
		element := r.typeFromSchema(v.AdditionalProperties)
		return schema.TypeSpec{
			Type:  v.Type,
			Items: &element,
		}
	}

	// We have found an inline object, so we need to generate that object, register it,
	// and then reference it.
	if v.Properties != nil {
		props := map[string]schema.PropertySpec{}
		required := []string{}
		for name, prop := range v.Properties {
			var req bool
			props[name], req = r.extendPath(name).propFromSchema(prop)
			if req {
				required = append(required, name)
			}
		}
		obj := schema.ObjectTypeSpec{
			Description: v.Description,
			Properties:  props,
			Type:        "object",
			Required:    required,
		}
		tk := r.extendPath(v.Title).register(schema.ComplexTypeSpec{
			ObjectTypeSpec: obj,
		})
		return schema.TypeSpec{Ref: tk}
	}

	if v.OneOf != nil {
		oneOf := make([]schema.TypeSpec, len(v.OneOf))
		for i, t := range v.OneOf {
			oneOf[i] = r.typeFromSchema(t)
		}
		return schema.TypeSpec{OneOf: oneOf}
	}

	// Other values can be added as needed.
	panic("unimplemented")
}

// Register a new type based on the current path. The token for the registered type is
// returned.
func (r registerTypes) register(typ schema.ComplexTypeSpec) string {
	mod := path.Base(path.Dir(r.path))
	item := path.Base(r.path)
	tk := fmt.Sprintf("pkg:%s:%s", mod, item)
	r.reg(tokens.Type(tk), typ)
	return tk
}

func (r registerTypes) propFromSchema(typ *openapi3.SchemaRef) (schema.PropertySpec, bool) {
	if typ.Ref != "" {
		panic("Ref not handled")
	}
	v := typ.Value
	var deprecated string
	if v.Deprecated {
		deprecated = "Deprecated"
	}
	return schema.PropertySpec{
		TypeSpec:           r.typeFromSchema(typ),
		Description:        v.Description,
		Default:            v.Default,
		DeprecationMessage: deprecated,
	}, !v.Nullable
}

func (op *Operation) schemaOutputs(reg s.RegisterDerivativeType) (properties, error) {
	props := &properties{
		props:    map[string]schema.PropertySpec{},
		required: codegen.NewStringSet(),
	}
	var errs multierror.Error
	responseRef, ok := op.Responses["200"]
	if !ok {
		return *props, fmt.Errorf("Could not find 200 response")
	}
	response := responseRef.Value
	err := registerTypes{
		path: op.Path,
		reg:  reg,
	}.extractTypes(reg, props, response.Content)
	if err != nil {
		errs.Errors = append(errs.Errors, err)
	}
	return *props, errs.ErrorOrNil()
}

func (r registerTypes) extractTypes(reg s.RegisterDerivativeType, props *properties, content openapi3.Content) error {
	if content == nil {
		return nil
	}
	c := content.Get("application/json")
	if c == nil {
		return fmt.Errorf("content does not support JSON")
	}
	if v := c.Schema; v != nil {
		if properties := v.Value.Properties; properties == nil {
			// If we get back a raw value, just emit it as a prop identified by then
			// encoding type.
			props.addProp("json", schema.PropertySpec{
				TypeSpec: r.typeFromSchema(v),
			})
		} else {
			// We got structured properties, so project them into an object.
			for name, prop := range properties {
				spec, required := r.propFromSchema(prop)
				props.addProp(name, spec)
				if required {
					props.required.Add(name)
				}
			}
		}
	}
	return nil
}
