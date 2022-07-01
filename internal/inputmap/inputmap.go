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

package inputmap

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type InputToImplementor map[reflect.Type]reflect.Type

func (m InputToImplementor) add(k interface{}, v interface{}) {
	m[reflect.TypeOf(k).Elem()] = reflect.TypeOf(v).Elem()
}

func GetInputMap() InputToImplementor {
	var inputMap InputToImplementor = make(map[reflect.Type]reflect.Type)
	//IntInput to int
	inputMap.add((*pulumi.IntInput)(nil), (*int)(nil))

	//IntPtrInput to *int
	inputMap.add((*pulumi.IntPtrInput)(nil), (**int)(nil))

	//IntArrayInput to []int
	inputMap.add((*pulumi.IntArrayInput)(nil), (*[]int)(nil))

	//IntMapInput to map[string]int
	inputMap.add((*pulumi.IntMapInput)(nil), (*map[string]int)(nil))

	//IntArrayMapInput to map[string][]int
	inputMap.add((*pulumi.IntArrayMapInput)(nil), (*map[string][]int)(nil))

	//IntMapArrayInput to []map[string]int
	inputMap.add((*pulumi.IntMapArrayInput)(nil), (*[]map[string]int)(nil))

	//IntMapMapInput to map[string]map[string]int
	inputMap.add((*pulumi.IntMapMapInput)(nil), (*map[string]map[string]int)(nil))

	//IntArrayArrayInput to [][]int
	inputMap.add((*pulumi.IntArrayArrayInput)(nil), (*[][]int)(nil))

	//StringInput to string
	inputMap.add((*pulumi.StringInput)(nil), (*string)(nil))

	//StringPtrInput to *string
	inputMap.add((*pulumi.StringPtrInput)(nil), (**string)(nil))

	//StringArrayInput to []string
	inputMap.add((*pulumi.StringArrayInput)(nil), (*[]string)(nil))

	//StringMapInput to map[string]string
	inputMap.add((*pulumi.StringMapInput)(nil), (*map[string]string)(nil))

	//StringArrayMapInput to map[string][]string
	inputMap.add((*pulumi.StringArrayMapInput)(nil), (*map[string][]string)(nil))

	//StringMapArrayInput to []map[string]string
	inputMap.add((*pulumi.StringMapArrayInput)(nil), (*[]map[string]string)(nil))

	//StringMapMapInput to map[string]map[string]string
	inputMap.add((*pulumi.StringMapMapInput)(nil), (*map[string]map[string]string)(nil))

	//StringArrayArrayInput to [][]string
	inputMap.add((*pulumi.StringArrayArrayInput)(nil), (*[][]string)(nil))

	//URNInput to pulumi.URN
	inputMap.add((*pulumi.URNInput)(nil), (*pulumi.URN)(nil))

	//URNPtrInput to *pulumi.URN
	inputMap.add((*pulumi.URNPtrInput)(nil), (**pulumi.URN)(nil))

	//URNArrayInput to []pulumi.URN
	inputMap.add((*pulumi.URNArrayInput)(nil), (*[]pulumi.URN)(nil))

	//URNMapInput to map[string]pulumi.URN
	inputMap.add((*pulumi.URNMapInput)(nil), (*map[string]pulumi.URN)(nil))

	//URNArrayMapInput to map[string][]pulumi.URN
	inputMap.add((*pulumi.URNArrayMapInput)(nil), (*map[string][]pulumi.URN)(nil))

	//URNMapArrayInput to []map[string]pulumi.URN
	inputMap.add((*pulumi.URNMapArrayInput)(nil), (*[]map[string]pulumi.URN)(nil))

	//URNMapMapInput to map[string]map[string]pulumi.URN
	inputMap.add((*pulumi.URNMapMapInput)(nil), (*map[string]map[string]pulumi.URN)(nil))

	//URNArrayArrayInput to [][]pulumi.URN
	inputMap.add((*pulumi.URNArrayArrayInput)(nil), (*[][]pulumi.URN)(nil))

	//ArchiveInput to pulumi.Archive
	inputMap.add((*pulumi.ArchiveInput)(nil), (*pulumi.Archive)(nil))

	//ArchiveArrayInput to []pulumi.Archive
	inputMap.add((*pulumi.ArchiveArrayInput)(nil), (*[]pulumi.Archive)(nil))

	//ArchiveMapInput to map[string]pulumi.Archive
	inputMap.add((*pulumi.ArchiveMapInput)(nil), (*map[string]pulumi.Archive)(nil))

	//ArchiveArrayMapInput to map[string][]pulumi.Archive
	inputMap.add((*pulumi.ArchiveArrayMapInput)(nil), (*map[string][]pulumi.Archive)(nil))

	//ArchiveMapArrayInput to []map[string]pulumi.Archive
	inputMap.add((*pulumi.ArchiveMapArrayInput)(nil), (*[]map[string]pulumi.Archive)(nil))

	//ArchiveMapMapInput to map[string]map[string]pulumi.Archive
	inputMap.add((*pulumi.ArchiveMapMapInput)(nil), (*map[string]map[string]pulumi.Archive)(nil))

	//ArchiveArrayArrayInput to [][]pulumi.Archive
	inputMap.add((*pulumi.ArchiveArrayArrayInput)(nil), (*[][]pulumi.Archive)(nil))

	//AssetInput to pulumi.Asset
	inputMap.add((*pulumi.AssetInput)(nil), (*pulumi.Asset)(nil))

	//AssetArrayInput to []pulumi.Asset
	inputMap.add((*pulumi.AssetArrayInput)(nil), (*[]pulumi.Asset)(nil))

	//AssetMapInput to map[string]pulumi.Asset
	inputMap.add((*pulumi.AssetMapInput)(nil), (*map[string]pulumi.Asset)(nil))

	//AssetArrayMapInput to map[string][]pulumi.Asset
	inputMap.add((*pulumi.AssetArrayMapInput)(nil), (*map[string][]pulumi.Asset)(nil))

	//AssetMapArrayInput to []map[string]pulumi.Asset
	inputMap.add((*pulumi.AssetMapArrayInput)(nil), (*[]map[string]pulumi.Asset)(nil))

	//AssetMapMapInput to map[string]map[string]pulumi.Asset
	inputMap.add((*pulumi.AssetMapMapInput)(nil), (*map[string]map[string]pulumi.Asset)(nil))

	//AssetArrayArrayInput to [][]pulumi.Asset
	inputMap.add((*pulumi.AssetArrayArrayInput)(nil), (*[][]pulumi.Asset)(nil))

	//AssetOrArchiveInput to pulumi.AssetOrArchive
	inputMap.add((*pulumi.AssetOrArchiveInput)(nil), (*pulumi.AssetOrArchive)(nil))

	//AssetOrArchiveArrayInput to []pulumi.AssetOrArchive
	inputMap.add((*pulumi.AssetOrArchiveArrayInput)(nil), (*[]pulumi.AssetOrArchive)(nil))

	//AssetOrArchiveMapInput to map[string]pulumi.AssetOrArchive
	inputMap.add((*pulumi.AssetOrArchiveMapInput)(nil), (*map[string]pulumi.AssetOrArchive)(nil))

	//AssetOrArchiveArrayMapInput to map[string][]pulumi.AssetOrArchive
	inputMap.add((*pulumi.AssetOrArchiveArrayMapInput)(nil), (*map[string][]pulumi.AssetOrArchive)(nil))

	//AssetOrArchiveMapArrayInput to []map[string]pulumi.AssetOrArchive
	inputMap.add((*pulumi.AssetOrArchiveMapArrayInput)(nil), (*[]map[string]pulumi.AssetOrArchive)(nil))

	//AssetOrArchiveMapMapInput to map[string]map[string]pulumi.AssetOrArchive
	inputMap.add((*pulumi.AssetOrArchiveMapMapInput)(nil), (*map[string]map[string]pulumi.AssetOrArchive)(nil))

	//AssetOrArchiveArrayArrayInput to [][]pulumi.AssetOrArchive
	inputMap.add((*pulumi.AssetOrArchiveArrayArrayInput)(nil), (*[][]pulumi.AssetOrArchive)(nil))

	//BoolInput to bool
	inputMap.add((*pulumi.BoolInput)(nil), (*bool)(nil))

	//BoolArrayInput to []bool
	inputMap.add((*pulumi.BoolArrayInput)(nil), (*[]bool)(nil))

	//BoolMapInput to map[string]bool
	inputMap.add((*pulumi.BoolMapInput)(nil), (*map[string]bool)(nil))

	//BoolArrayMapInput to map[string][]bool
	inputMap.add((*pulumi.BoolArrayMapInput)(nil), (*map[string][]bool)(nil))

	//BoolMapArrayInput to []map[string]bool
	inputMap.add((*pulumi.BoolMapArrayInput)(nil), (*[]map[string]bool)(nil))

	//BoolMapMapInput to map[string]map[string]bool
	inputMap.add((*pulumi.BoolMapMapInput)(nil), (*map[string]map[string]bool)(nil))

	//BoolArrayArrayInput to [][]bool
	inputMap.add((*pulumi.BoolArrayArrayInput)(nil), (*[][]bool)(nil))

	//IDInput to pulumi.ID
	inputMap.add((*pulumi.IDInput)(nil), (*pulumi.ID)(nil))

	//IDPtrInput to *pulumi.ID
	inputMap.add((*pulumi.IDPtrInput)(nil), (**pulumi.ID)(nil))

	//IDArrayInput to []pulumi.ID
	inputMap.add((*pulumi.IDArrayInput)(nil), (*[]pulumi.ID)(nil))

	//IDMapInput to map[string]pulumi.ID
	inputMap.add((*pulumi.IDMapInput)(nil), (*map[string]pulumi.ID)(nil))

	//IDArrayMapInput to map[string][]pulumi.ID
	inputMap.add((*pulumi.IDArrayMapInput)(nil), (*map[string][]pulumi.ID)(nil))

	//IDMapArrayInput to []map[string]pulumi.ID
	inputMap.add((*pulumi.IDMapArrayInput)(nil), (*[]map[string]pulumi.ID)(nil))

	//IDMapMapInput to map[string]map[string]pulumi.ID
	inputMap.add((*pulumi.IDMapMapInput)(nil), (*map[string]map[string]pulumi.ID)(nil))

	//IDArrayArrayInput to [][]pulumi.ID
	inputMap.add((*pulumi.IDArrayArrayInput)(nil), (*[][]pulumi.ID)(nil))

	//ArrayInput to []interface{}
	inputMap.add((*pulumi.ArrayInput)(nil), (*[]interface{})(nil))

	//MapInput to map[string]interface{}
	inputMap.add((*pulumi.MapInput)(nil), (*map[string]interface{})(nil))

	//ArrayMapInput to map[string][]interface{}
	inputMap.add((*pulumi.ArrayMapInput)(nil), (*map[string][]interface{})(nil))

	//MapArrayInput to []map[string]interface{}
	inputMap.add((*pulumi.MapArrayInput)(nil), (*[]map[string]interface{})(nil))

	//MapMapInput to map[string]map[string]interface{}
	inputMap.add((*pulumi.MapMapInput)(nil), (*map[string]map[string]interface{})(nil))

	//ArrayArrayInput to [][]interface{}
	inputMap.add((*pulumi.ArrayArrayInput)(nil), (*[][]interface{})(nil))

	//ArrayArrayMapInput to map[string][][]interface{}
	inputMap.add((*pulumi.ArrayArrayMapInput)(nil), (*map[string][][]interface{})(nil))

	//Float65Input to float64
	inputMap.add((*pulumi.Float64Input)(nil), (*float64)(nil))

	//Float64PtrInput to *float64
	inputMap.add((*pulumi.Float64PtrInput)(nil), (**float64)(nil))

	//Float64ArrayInput to []float64
	inputMap.add((*pulumi.Float64ArrayInput)(nil), (*[]float64)(nil))

	//Float64MapInput to map[string]float64
	inputMap.add((*pulumi.Float64MapInput)(nil), (*map[string]float64)(nil))

	//Float64ArrayMapInput to map[string][]float64
	inputMap.add((*pulumi.Float64ArrayMapInput)(nil), (*map[string][]float64)(nil))

	//Float64MapArrayInput to []map[string]float64
	inputMap.add((*pulumi.Float64MapArrayInput)(nil), (*[]map[string]float64)(nil))

	//Float64MapMapInput to map[string]map[string]float64
	inputMap.add((*pulumi.Float64MapMapInput)(nil), (*map[string]map[string]float64)(nil))

	//Float64ArrayArrayInput to [][]float64
	inputMap.add((*pulumi.Float64ArrayArrayInput)(nil), (*[][]float64)(nil))

	//ResourceInput to pulumi.Resource
	inputMap.add((*pulumi.ResourceInput)(nil), (*pulumi.Resource)(nil))

	//ResourceArrayInput to []pulumi.Resource
	inputMap.add((*pulumi.ResourceArrayInput)(nil), (*[]pulumi.Resource)(nil))

	return inputMap
}
