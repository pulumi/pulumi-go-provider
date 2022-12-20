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
