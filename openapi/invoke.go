package openapi

import p "github.com/pulumi/pulumi-go-provider"

type Invoke struct {
	Call *Operation

	CheckFunc func(p.Context, p.CheckRequest) (p.CheckResponse, error)
}
