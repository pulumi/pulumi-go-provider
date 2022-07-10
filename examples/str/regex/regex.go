package regex

import "regexp"

func Replace(input ReplaceIn) (Ret, error) {
	r, err := regexp.Compile(input.Old)
	if err != nil {
		return Ret{}, err
	}
	result := r.ReplaceAllLiteralString(input.S, input.New)
	return Ret{result}, nil
}

type ReplaceIn struct {
	S   string `pulumi:"s"`
	Old string `pulumi:"old"`
	New string `pulumi:"new"`
}

type Ret struct {
	Out string `pulumi:"out"`
}
