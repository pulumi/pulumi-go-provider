package regex

import (
	"context"
	"regexp"

	"github.com/pulumi/pulumi-go-provider/infer"
)

type Replace struct{}

func (Replace) Call(_ context.Context, args ReplaceArgs) (Ret, error) {
	r, err := regexp.Compile(args.Pattern)
	if err != nil {
		return Ret{}, err
	}
	result := r.ReplaceAllLiteralString(args.S, args.New)
	return Ret{result}, nil
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
