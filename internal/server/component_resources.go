package server

import (
	"fmt"

	"github.com/pulumi/pulumi-go-provider/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type Components map[URN]resource.Component

// Return a fully hydrated component.
func (c Components) GetComponent(request pulumirpc.ConstructRequest) (resource.Component, error) {
	return nil, fmt.Errorf("Unimplemented")
}

func serializeComponent(c resource.Component) (pulumirpc.ConstructResponse, error) {
	return pulumirpc.ConstructResponse{}, nil
}
