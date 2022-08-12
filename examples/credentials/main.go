package main

import (
	"fmt"
	"os"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func main() {
	err := p.RunProvider("credentials", semver.Version{Minor: 1}, provider())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}

func provider() p.Provider {
	return infer.NewProvider().
		WithResources(infer.Resource[*User, UserArgs, UserState]()).
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{
			"credentials": "index",
		}).
		WithConfig(infer.Config[*Config]())
}

type Config struct {
	User     string `pulumi:"user"`
	Password string `pulumi:"password" provider:"secret"`
}

var _ = (infer.Annotated)((*Config)(nil))

func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(&c.User, "The username. Its important but not secret.")
	a.Describe(&c.Password, "The password. It is very secret.")
	a.SetDefault(&c.Password, "", "FOO")
}

type User struct{}
type UserArgs struct{}
type UserState struct {
	Value string `pulumi:"value"`
}

func (*User) Create(ctx p.Context, name string, input UserArgs, preview bool) (string, UserState, error) {
	return name, UserState{
		infer.GetConfig[Config](ctx).User,
	}, nil
}

var _ = (infer.CustomDiff[UserArgs, UserState])((*User)(nil))

func (*User) Diff(ctx p.Context, id string, olds UserState, news UserArgs) (p.DiffResponse, error) {
	if infer.GetConfig[Config](ctx).User != olds.Value {
		return p.DiffResponse{
			HasChanges: true,
			DetailedDiff: map[string]p.PropertyDiff{
				"value": {Kind: p.UpdateReplace},
			},
		}, nil
	}
	return p.DiffResponse{}, nil
}
