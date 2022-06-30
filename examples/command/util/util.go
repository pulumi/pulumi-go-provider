package util

import (
	"bufio"
	"io"

	r "github.com/pulumi/pulumi-go-provider/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
)

func CopyOutput(ctx r.Context, r io.Reader, doneCh chan<- struct{}, severity diag.Severity) {
	defer close(doneCh)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		err := ctx.Log(severity, "%s", scanner.Text())
		if err != nil {
			return
		}
	}
}
