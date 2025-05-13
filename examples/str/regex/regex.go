// Copyright 2025, Pulumi Corporation.  All rights reserved.

package regex

import (
	"context"
	"regexp"

	"github.com/pulumi/pulumi-go-provider/infer"
)

type Replace struct{}

func (Replace) Invoke(_ context.Context, req infer.FunctionRequest[ReplaceArgs]) (infer.FunctionResponse[Ret], error) {
	r, err := regexp.Compile(req.Input.Pattern)
	if err != nil {
		return infer.FunctionResponse[Ret]{}, err
	}
	result := r.ReplaceAllLiteralString(req.Input.S, req.Input.New)
	return infer.FunctionResponse[Ret]{
		Output: Ret{result},
	}, nil
}

func (r *Replace) Annotate(a infer.Annotator) {
	a.Describe(r,
		"Replace returns a copy of `s`, replacing matches of the `old`\n"+
			"with the replacement string `new`.")
}

type ReplaceArgs struct {
	S       string `pulumi:"s"`
	Pattern string `pulumi:"pattern"`
	New     string `pulumi:"new"`
}

type Ret struct {
	Out string `pulumi:"out"`
}
