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
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

func TestUnderlyingType(t *testing.T) {
	t.Parallel()
	intPtrInputType := reflect.TypeOf((*pulumi.IntPtrInput)(nil)).Elem()
	intPtrOutputType := reflect.TypeOf((*pulumi.IntPtrOutput)(nil)).Elem()
	intType := reflect.TypeOf((*int)(nil)).Elem()
	intPtrInputUnderlying, err := underlyingType(intPtrInputType)
	assert.NoError(t, err)
	intPtrOutputUnderlying, err := underlyingType(intPtrOutputType)
	assert.NoError(t, err)
	assert.Equal(t, intType, intPtrInputUnderlying)
	assert.Equal(t, intType, intPtrOutputUnderlying)

	assetArrayArrayInputType := reflect.TypeOf((*pulumi.AssetArrayArrayInput)(nil)).Elem()
	assetArrayArrayOutputType := reflect.TypeOf((*pulumi.AssetArrayArrayOutput)(nil)).Elem()
	assetArrayArrayType := reflect.TypeOf((*[][]pulumi.Asset)(nil)).Elem()
	assetArrayArrayInputUnderlying, err := underlyingType(assetArrayArrayInputType)
	assert.NoError(t, err)
	assetArrayArrayOutputUnderlying, err := underlyingType(assetArrayArrayOutputType)
	assert.NoError(t, err)
	assert.Equal(t, assetArrayArrayType, assetArrayArrayInputUnderlying)
	assert.Equal(t, assetArrayArrayType, assetArrayArrayOutputUnderlying)

	stringMapArrayInputType := reflect.TypeOf((*pulumi.StringMapArrayInput)(nil)).Elem()
	stringMapArrayOutputType := reflect.TypeOf((*pulumi.StringMapArrayOutput)(nil)).Elem()
	stringMapArrayType := reflect.TypeOf((*[]map[string]string)(nil)).Elem()
	stringMapArrayInputUnderlying, err := underlyingType(stringMapArrayInputType)
	assert.NoError(t, err)
	stringMapArrayOutputUnderlying, err := underlyingType(stringMapArrayOutputType)
	assert.NoError(t, err)
	assert.Equal(t, stringMapArrayType, stringMapArrayInputUnderlying)
	assert.Equal(t, stringMapArrayType, stringMapArrayOutputUnderlying)

	amiCopyEbsBlockDeviceArrayInput := reflect.TypeOf((*ec2.AmiCopyEbsBlockDeviceArrayInput)(nil)).Elem()
	amiCopyEbsBlockDeviceArrayOutput := reflect.TypeOf((*ec2.AmiCopyEbsBlockDeviceArrayOutput)(nil)).Elem()
	amiCopyEbsBlockDeviceArrayType := reflect.TypeOf((*[]ec2.AmiCopyEbsBlockDevice)(nil)).Elem()
	amiCopyEbsBlockDeviceArrayInputUnderlying, err := underlyingType(amiCopyEbsBlockDeviceArrayInput)
	assert.NoError(t, err)
	amiCopyEbsBlockDeviceArrayOutputUnderlying, err := underlyingType(amiCopyEbsBlockDeviceArrayOutput)
	assert.NoError(t, err)
	assert.Equal(t, amiCopyEbsBlockDeviceArrayType, amiCopyEbsBlockDeviceArrayInputUnderlying)
	assert.Equal(t, amiCopyEbsBlockDeviceArrayType, amiCopyEbsBlockDeviceArrayOutputUnderlying)

	manyPtrIntType := reflect.TypeOf((*****int)(nil)).Elem()
	manyPtrIntUnderlying, err := underlyingType(manyPtrIntType)
	assert.NoError(t, err)
	intType = reflect.TypeOf(5)
	assert.NoError(t, err)
	assert.Equal(t, intType, manyPtrIntUnderlying)

	assert.NotEqual(t, stringMapArrayInputUnderlying, amiCopyEbsBlockDeviceArrayInputUnderlying)
}
