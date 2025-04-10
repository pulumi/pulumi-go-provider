package main

import (
	"context"
	"fmt"
	"hash/adler32"
	"hash/crc32"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func main() {
	err := p.RunProvider("credentials", "0.1.0", provider())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}

func provider() p.Provider {
	return infer.Provider(infer.Options{
		Resources: []infer.InferredResource{infer.Resource[*User]()},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{
			"credentials": "index",
		},
		Config: infer.Config[*Config](),
		Functions: []infer.InferredFunction{
			infer.Function[*Sign](),
		},
	})
}

type Config struct {
	User     string   `pulumi:"user"`
	Password string   `pulumi:"password,optional" provider:"secret"`
	HashKind HashKind `pulumi:"hash"`

	hashedPassword string
}

type HashKind string

var _ = (infer.Enum[HashKind])((*HashKind)(nil))

const (
	HashAdler HashKind = "Adler32"
	HashCRC   HashKind = "CRC32"
)

func (*HashKind) Values() []infer.EnumValue[HashKind] {
	return []infer.EnumValue[HashKind]{
		{Value: HashAdler, Description: "Adler32 implements the Adler-32 checksum."},
		{Value: HashCRC, Description: "CRC32 implements the 32-bit cyclic redundancy check, or CRC-32, checksum."},
	}
}

var _ = (infer.Annotated)((*Config)(nil))

func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(&c.User, "The username. Its important but not secret.")
	a.Describe(&c.Password, "The password. It is very secret.")
	a.Describe(&c.HashKind, `The (entirely uncryptographic) hash function used to encode the "password".`)
	a.SetDefault(&c.Password, "", "FOO")
	a.SetDefault(&c.HashKind, HashAdler)
}

var _ = (infer.CustomConfigure)((*Config)(nil))

func (c *Config) Configure(ctx context.Context) error {
	msg := fmt.Sprintf("credentials provider setup with user: %q", c.User)
	if c.Password != "" {
		msg += fmt.Sprintf(" and a very secret password (its %q)", c.Password)
	}
	switch c.HashKind {
	case HashAdler:
		c.hashedPassword = fmt.Sprintf("%d", adler32.Checksum([]byte(c.Password)))
	case HashCRC:
		c.hashedPassword = fmt.Sprintf("%d", crc32.ChecksumIEEE([]byte(c.Password)))
	}
	p.GetLogger(ctx).Info(msg)
	return nil
}

type User struct{}
type UserArgs struct{}
type UserState struct {
	Name     string `pulumi:"name"`
	Password string `pulumi:"password"`
}

func (*User) Create(ctx context.Context, req infer.CreateRequest[UserArgs]) (infer.CreateResponse[UserState], error) {
	config := infer.GetConfig[Config](ctx)
	return infer.CreateResponse[UserState]{
		ID: req.Name,
		Output: UserState{
			Name:     config.User,
			Password: config.hashedPassword,
		}}, nil
}

var _ = (infer.CustomDiff[UserArgs, UserState])((*User)(nil))

func (*User) Diff(ctx context.Context, req infer.DiffRequest[UserArgs, UserState]) (infer.DiffResponse, error) {
	config := infer.GetConfig[Config](ctx)
	if config.User != req.Olds.Name {
		return infer.DiffResponse{
			HasChanges: true,
			DetailedDiff: map[string]p.PropertyDiff{
				"name": {Kind: p.UpdateReplace},
			},
		}, nil
	}
	return p.DiffResponse{}, nil
}

type Sign struct{}

func (Sign) Invoke(ctx context.Context, req infer.FunctionRequest[SignArgs]) (infer.FunctionResponse[SignRes], error) {
	config := infer.GetConfig[Config](ctx)
	return infer.FunctionResponse[SignRes]{
		Output: SignRes{
			Out: fmt.Sprintf("%s by %s", req.Input.Message, config.User),
		},
	}, nil
}

func (r *Sign) Annotate(a infer.Annotator) {
	a.Describe(r, "Signs the message with the user name and returns the result as a secret.")
}

type SignArgs struct {
	Message string `pulumi:"message"`
}

func (ra *SignArgs) Annotate(a infer.Annotator) {
	a.Describe(&ra.Message, "Message to sign.")
}

type SignRes struct {
	Out string `pulumi:"out" provider:"secret"`
}
