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

package infer

import (
	"context"
	"testing"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFnTokens(t *testing.T) {
	t.Parallel()
	cases := []struct{ input, expected tokens.Type }{
		{"pkg:foo:Run", "pkg:foo:run"},
		{"pkg:foo:ToJSON", "pkg:foo:toJSON"},
		{"pkg:foo:FOO", "pkg:foo:foo"},
		{"pkg:foo:FIZZBuzz", "pkg:foo:fizzBuzz"},
	}

	for _, c := range cases {
		t.Run(string(c.input), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.expected, fnToken(c.input))
		})
	}

}

type deprecatedFn struct{}

type deprecatedFnArgs struct{}

func (deprecatedFn) Invoke(
	context.Context, FunctionRequest[deprecatedFnArgs],
) (FunctionResponse[deprecatedFnArgs], error) {
	return FunctionResponse[deprecatedFnArgs]{}, nil
}

func (f *deprecatedFn) Annotate(a Annotator) {
	a.Describe(&f, "Does a thing.")
	a.Deprecate(&f, "Use otherFunction instead.")
}

func TestFunctionSchemaDeprecation(t *testing.T) {
	t.Parallel()

	spec, err := Function(deprecatedFn{}).GetSchema(
		func(tokens.Type, pschema.ComplexTypeSpec) bool { return false },
	)
	require.NoError(t, err)

	assert.Equal(t, pschema.FunctionSpec{
		Description:        "Does a thing.",
		DeprecationMessage: "Use otherFunction instead.",
		Inputs:             &pschema.ObjectTypeSpec{Properties: map[string]pschema.PropertySpec{}, Type: "object"},
		Outputs:            &pschema.ObjectTypeSpec{Properties: map[string]pschema.PropertySpec{}, Type: "object"},
	}, spec)
}
