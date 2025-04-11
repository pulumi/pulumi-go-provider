package integration

import (
	_ "unsafe" // unsafe is needed to use go:linkname

	structpb "github.com/golang/protobuf/ptypes/struct"
	p "github.com/pulumi/pulumi-go-provider"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type propertyToRPC func(m presource.PropertyMap) (*structpb.Struct, error)

type propertyFromRPC func(m *structpb.Struct) (presource.PropertyMap, error)

//go:linkname linkedConstructRequestToRPC github.com/pulumi/pulumi-go-provider.linkedConstructRequestToRPC
func linkedConstructRequestToRPC(req *p.ConstructRequest, marshal propertyToRPC) *rpc.ConstructRequest

//go:linkname linkedConstructResponseFromRPC github.com/pulumi/pulumi-go-provider.linkedConstructResponseFromRPC
func linkedConstructResponseFromRPC(resp *rpc.ConstructResponse, unmarshal propertyFromRPC) (p.ConstructResponse, error)
