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

package provider

import (
	"reflect"
	"testing"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/eks"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

type TestStruct struct {
	wrapper  any
	base     any
	notequal bool
}

func TestUnderlyingType(t *testing.T) {
	t.Parallel()
	tests := []TestStruct{
		{
			wrapper: (*pulumi.IntPtrInput)(nil),
			base:    (*int)(nil),
		},
		{
			wrapper: (*pulumi.IntPtrOutput)(nil),
			base:    (*int)(nil),
		},
		{
			wrapper: (*pulumi.AssetArrayArrayInput)(nil),
			base:    (*[][]pulumi.Asset)(nil),
		},
		{
			wrapper: (*ec2.AmiCopyEbsBlockDeviceArrayInput)(nil),
			base:    (*[]ec2.AmiCopyEbsBlockDevice)(nil),
		},
		{
			wrapper: (*eks.ClusterOutput)(nil),
			base:    (*eks.Cluster)(nil),
		},
		{
			wrapper: (*pulumi.StringArrayInput)(nil),
			base:    (*[]string)(nil),
		},
		{
			wrapper: (****int)(nil),
			base:    (*int)(nil),
		},
		{
			wrapper:  (*int)(nil),
			base:     (*string)(nil),
			notequal: true,
		},
	}

	for _, test := range tests {
		wrapper := reflect.TypeOf(test.wrapper).Elem()
		base := reflect.TypeOf(test.base).Elem()
		underlying, err := underlyingType(wrapper)
		assert.NoError(t, err)
		if test.notequal {
			assert.NotEqual(t, base, underlying)
		} else {
			assert.Equal(t, base.Name(), underlying.Name())
		}
	}
}
