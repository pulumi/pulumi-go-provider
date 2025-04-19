// Copyright 2025, Pulumi Corporation.
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

package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	r "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func TestConstruct(t *testing.T) {
	t.Parallel()

	prov := providerWithMocks[Config](t, &integration.MockMonitor{
		NewResourceF: func(args pulumi.MockResourceArgs) (string, r.PropertyMap, error) {
			assert.Equal(t, "test:index:RandomComponent", args.TypeToken)
			assert.Equal(t, "test-component", args.Name)
			return args.ID, r.PropertyMap{}, nil
		},
	})

	prefix := r.NewProperty(r.Output{
		Secret:       true,
		Dependencies: []r.URN{urn("Other", "other")},
		Known:        true,
		Element:      r.NewStringProperty("foo-"),
	})

	resp, err := prov.Construct(p.ConstructRequest{
		Urn:    childUrn("RandomComponent", "test-component", "test-parent"),
		Parent: urn("Parent", "test-parent"),
		Inputs: r.PropertyMap{
			"prefix": prefix,
		},
		InputDependencies: map[r.PropertyKey][]r.URN{
			"prefix": {urn("Other", "more")},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, r.URN("urn:pulumi:stack::project::test:index:Parent$test:index:RandomComponent::test-component"),
		resp.Urn)

	assert.Equal(t, r.PropertyMap{
		"result": r.MakeSecret(r.NewStringProperty("foo-12345")),
	}, resp.State)
}
