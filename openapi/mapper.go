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

package openapi

import (
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type Mapper struct {
	doc *openapi3.T

	// Backlinks within the document
	opPaths   map[*openapi3.Operation]*openapi3.PathItem
	pathNames map[*openapi3.PathItem]string
}

func New(doc *openapi3.T) Mapper {
	contract.Assert(doc != nil)
	// Pre-compute back paths
	opPaths := map[*openapi3.Operation]*openapi3.PathItem{}
	pathNames := map[*openapi3.PathItem]string{}
	for name, path := range doc.Paths {
		pathNames[path] = name
		for _, op := range path.Operations() {
			opPaths[op] = path
		}
	}
	return Mapper{
		doc:       doc,
		opPaths:   opPaths,
		pathNames: pathNames,
	}
}

func (m *Mapper) NewOperation(op *openapi3.Operation) *Operation {
	item, ok := m.opPaths[op]
	contract.Assertf(ok, "Operation not found in the original document")
	path, ok := m.pathNames[item]
	contract.Assertf(ok, "PathItem not found in the original document")
	return &Operation{
		Operation: *op,
		doc:       m.doc,
		pathItem:  item,
		path:      path,
	}
}
