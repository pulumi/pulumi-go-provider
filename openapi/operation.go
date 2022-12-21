package openapi

import (
	"fmt"
	"io"
	"net/http"
	netUrl "net/url"
	"path"
	"reflect"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/hashicorp/go-multierror"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	res "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	p "github.com/pulumi/pulumi-go-provider"
	s "github.com/pulumi/pulumi-go-provider/middleware/schema"
)

var DefaultClient = new(http.Client)

type Operation struct {
	openapi3.Operation

	Client *http.Client

	doc      *openapi3.T
	pathItem *openapi3.PathItem
	path     string

	mapping Mappings
}

type url struct {
	path   string
	params netUrl.Values
}

func (u url) build() string {
	var params string
	if len(u.params) > 0 {
		params = "?" + u.params.Encode()
	}
	return u.path + params
}

func (u *url) replace(name, value string) {
	u.path = strings.ReplaceAll(u.path, "{"+name+"}", value)
}

func (u *url) query(name, value string) {
	if u.params == nil {
		u.params = netUrl.Values{}
	}
	u.params.Add(name, value)
}

func (op *Operation) url() (url, error) {
	var base string
	var err error
	if op.pathItem.Servers != nil {
		base, err = op.pathItem.Servers.BasePath()
	} else {
		base, err = op.doc.Servers.BasePath()
	}
	if err != nil {
		return url{}, err
	}
	return url{path: base + op.path}, nil
}

type body map[string]interface{}

func (op *Operation) body() body {
	return body{}
}

func (b body) build() io.Reader {
	return nil
}

func (op *Operation) method() string {
	for method, i := range op.pathItem.Operations() {
		if i == &op.Operation {
			return method
		}
	}
	panic("Operation not found in associated pathItem")
}

type properties struct {
	props    map[string]schema.PropertySpec
	rawTypes map[string]*openapi3.Schema
	required codegen.StringSet
}

func (p *properties) addProp(name string, prop schema.PropertySpec, raw *openapi3.Schema, required bool) {
	if p.props == nil {
		p.props = map[string]schema.PropertySpec{}
	}
	p.props[name] = prop

	if p.rawTypes == nil {
		p.rawTypes = map[string]*openapi3.Schema{}
	}
	p.rawTypes[name] = raw

	if required {
		if p.required == nil {
			p.required = codegen.NewStringSet()
		}
		p.required.Add(name)
	}
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

func (op *Operation) schemaInputs(resource *Resource, reg s.RegisterDerivativeType) (properties, error) {
	props := properties{
		required: codegen.NewStringSet(),
	}
	r := op.register(resource, reg)

	for _, paramRef := range op.Parameters {
		param := paramRef.Value

		if op.mapping.targets(resource, op, param.Name) {
			continue
		}
		var err error
		if param.Schema != nil {
			err = r.addProp(&props, param.Name, param.Schema, true)
		} else {
			err = r.extractTypes(&props, param.Content, true)
		}
		if err != nil {
			return props, err
		}

		spec, ok := props.props[param.Name]
		if ok {
			if param.Deprecated {
				spec.DeprecationMessage = "This resource is depreciated."
			}
			if param.Description != "" {
				spec.Description = param.Description
			}
			props.props[param.Name] = spec
		}
	}

	var err error
	if body := op.Operation.RequestBody; body != nil {
		err = r.extractTypes(&props, body.Value.Content, true)
	}
	return props, err
}

type registerTypes struct {
	op       *Operation
	path     string
	reg      s.RegisterDerivativeType
	resource *Resource
}

func (r registerTypes) extendPath(segment string) registerTypes {
	r.path += "/" + segment
	return r
}

func (r registerTypes) typeFromSchemaValue(v *openapi3.Schema) schema.TypeSpec {
	// We have a value, so we try to infer the type
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

func (r registerTypes) typeFromSchema(typ *openapi3.SchemaRef) schema.TypeSpec {
	if typ == nil {
		return schema.TypeSpec{Ref: "pulumi.json#/Any"}
	}

	// if typ.Ref != "" {
	// 	TODO: Does it matter that its a ref?
	// }
	return r.typeFromSchemaValue(typ.Value)
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

func (op *Operation) register(resource *Resource, reg s.RegisterDerivativeType) registerTypes {
	id := strings.Split(op.OperationID, "_")
	return registerTypes{
		op:       op,
		path:     op.path,
		reg:      reg,
		resource: resource,
	}.extendPath(id[len(id)-1])
}

func (op *Operation) schemaOutputs(resource *Resource, reg s.RegisterDerivativeType) (properties, error) {
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
	err := op.register(resource, reg).extractTypes(props, response.Content, false)
	if err != nil {
		errs.Errors = append(errs.Errors, err)
	}
	return *props, errs.ErrorOrNil()
}

func (r registerTypes) extractTypes(props *properties, content openapi3.Content, input bool) error {
	if content == nil {
		return nil
	}
	c := content.Get("application/json")
	if c == nil {
		if len(content) == 0 {
			// Nothing but the code returned, so no types to extract.
			return nil
		}
		// We won't be able to decode, but other content types exist.
		return fmt.Errorf(
			"%s: content does not support JSON, but does support other types",
			r.path)
	}

	var errs multierror.Error
	if v := c.Schema; v != nil {
		if properties := v.Value.Properties; properties == nil {
			// If we get back a raw value, just emit it as a prop identified by then
			// encoding type.
			props.addProp("json", schema.PropertySpec{
				TypeSpec: r.typeFromSchema(v),
			}, v.Value, true)
		} else {
			// We got structured properties, so project them into an object.
			for name, prop := range properties {
				err := r.addProp(props, name, prop, input)
				if err != nil {
					errs.Errors = append(errs.Errors, err)
				}
			}
		}
	}
	return errs.ErrorOrNil()
}

func (r *registerTypes) addProp(props *properties, name string, prop *openapi3.SchemaRef, input bool) error {
	if (prop.Value.ReadOnly && input) ||
		(prop.Value.WriteOnly && !input) ||
		r.op.mapping.targets(r.resource, r.op, name) {
		return nil
	}
	spec, required := r.propFromSchema(prop)
	val := prop.Value
	props.addProp(name, spec, val, required)
	return nil
}
