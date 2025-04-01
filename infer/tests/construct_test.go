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

package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	r "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestConstruct(t *testing.T) {
	t.Parallel()

	t.Run("up", func(t *testing.T) {
		t.Parallel()

		prov := providerWithMocks[Config](t, &integration.MockMonitor{
			NewResourceF: func(args pulumi.MockResourceArgs) (string, r.PropertyMap, error) {
				assert.Equal(t, "up", args.Name)
				// assert.Equal(t, "up", args.Inputs["name"].StringValue())
				return "up-id", r.PropertyMap{}, nil
			},
		})

		resp, err := prov.Construct(p.ConstructRequest{
			Urn: urn("ReadConfigComponent", "up"),
			ConstructRequest: &rpc.ConstructRequest{
				Project: "test-project",
				Stack:   "test-stack",
			},

			// Properties: resource.PropertyMap{
			// 	"string": resource.NewStringProperty("foo"),
			// 	"int":    resource.NewNumberProperty(4.0),
			// },
		})
		assert.NoError(t, err)
		assert.Equal(t, "urn:pulumi:stack::project::test:index:ReadConfigComponent::up", resp.Urn)
		// assert.Equal(t, resource.PropertyMap{
		// 	"name":         s("(up)"),
		// 	"stringPlus":   s("foo+"),
		// 	"stringAndInt": s("foo-4"),
		// }, resp.Properties)
	})
}
