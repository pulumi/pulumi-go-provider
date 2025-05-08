// Copyright 2025, Pulumi Corporation.  All rights reserved.

// This is intended to be an example of the enclosing SDK. Do not use this for
// cryptography.
package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	randomlogin "github.com/pulumi/pulumi-go-provider/examples/random-login/sdk/go/randomlogin"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	goGen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
)

func main() {
	provider, err := provider()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
	err = provider.Run(context.Background(), "random-login", "0.1.0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}

func provider() (p.Provider, error) {
	return infer.NewProviderBuilder().
		WithNamespace("examples").
		WithResources(
			infer.Resource(&RandomSalt{}),
		).
		WithComponents(
			infer.Component(&RandomLogin{}),
			infer.ComponentF(NewMoreRandomPassword),
		).
		WithConfig(infer.Config(Config{})).
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{
			"random-login": "index",
		}).
		WithLanguageMap(map[string]any{
			"go": goGen.GoPackageInfo{
				ImportBasePath: "github.com/pulumi/pulumi-go-provider/examples/random-login/sdk/go/randomlogin",
			},
		}).
		Build()
}

type MoreRandomPasswordArgs struct {
	Length pulumi.IntInput `pulumi:"length"`
}

type MoreRandomPassword struct {
	pulumi.ResourceState
	Length   pulumi.IntOutput    `pulumi:"length"`
	Password pulumi.StringOutput `pulumi:"password"`
}

func (l *MoreRandomPassword) Annotate(a infer.Annotator) {
	a.Describe(&l, "Generate a random password.")
}

func (l *MoreRandomPasswordArgs) Annotate(a infer.Annotator) {
	a.Describe(&l.Length, "The desired password length.")
}

func NewMoreRandomPassword(ctx *pulumi.Context, name string, args *MoreRandomPasswordArgs, opts ...pulumi.ResourceOption) (*MoreRandomPassword, error) {
	comp := &MoreRandomPassword{
		Length: args.Length.ToIntOutput(),
	}
	err := ctx.RegisterComponentResource(p.GetTypeToken(ctx), name, comp, opts...)
	if err != nil {
		return nil, err
	}

	pArgs := &random.RandomPasswordArgs{
		Length: args.Length,
	}

	config := infer.GetConfig[Config](ctx.Context())
	if config.Scream != nil {
		pArgs.Lower = pulumi.BoolPtr(*config.Scream)
	}

	password, err := random.NewRandomPassword(ctx, name+"-password", pArgs, pulumi.Parent(comp))
	if err != nil {
		return nil, err
	}
	comp.Password = password.Result

	return comp, nil
}

type RandomLogin struct{}

type RandomLoginArgs struct {
	PetName bool `pulumi:"petName"`
}

type RandomLoginState struct {
	pulumi.ResourceState
	RandomLoginArgs

	Username pulumi.StringOutput `pulumi:"username"`
	Password pulumi.StringOutput `pulumi:"password"`
}

func (r *RandomLogin) Construct(ctx *pulumi.Context, name, typ string, args RandomLoginArgs, opts pulumi.ResourceOption) (*RandomLoginState, error) {
	comp := &RandomLoginState{}
	err := ctx.RegisterComponentResource(typ, name, comp, opts)
	if err != nil {
		return nil, err
	}
	if args.PetName {
		pet, err := random.NewRandomPet(ctx, name+"-pet", &random.RandomPetArgs{}, pulumi.Parent(comp))
		if err != nil {
			return nil, err
		}
		comp.Username = pet.ID().ToStringOutput()
	} else {
		id, err := random.NewRandomId(ctx, name+"-id", &random.RandomIdArgs{
			ByteLength: pulumi.Int(8),
		}, pulumi.Parent(comp))
		if err != nil {
			return nil, err
		}
		comp.Username = id.ID().ToStringOutput()
	}

	// create a variable-length password using a nested component
	length, err := random.NewRandomInteger(ctx, name+"-length", &random.RandomIntegerArgs{
		Min: pulumi.Int(8),
		Max: pulumi.Int(24),
	}, pulumi.Parent(comp))
	if err != nil {
		return nil, err
	}
	password, err := randomlogin.NewMoreRandomPassword(ctx, name+"-password", &randomlogin.MoreRandomPasswordArgs{
		Length: length.Result,
	}, pulumi.Parent(comp))
	if err != nil {
		return nil, err
	}
	comp.Password = password.Password

	return comp, nil
}

func (l *RandomLogin) Annotate(a infer.Annotator) {
	a.Describe(&l, "Generate a random login.")
	a.AddAlias("other", "RandomLogin")
}

func (l *RandomLoginArgs) Annotate(a infer.Annotator) {
	a.Describe(&l.PetName, "Whether to use a memorable pet name or a random string for the Username.")
}

func (l *RandomLoginState) Annotate(a infer.Annotator) {
	a.Describe(&l.Username, "The generated username.")
	a.Describe(&l.Password, "The generated password.")
}

type RandomSalt struct{}

type RandomSaltState struct {
	Salt           string `pulumi:"salt"`
	SaltedPassword string `pulumi:"saltedPassword"`
	Password       string `pulumi:"password"`
	SaltLength     *int   `pulumi:"saltedLength,optional"`
}

type RandomSaltArgs struct {
	Password   string `pulumi:"password"`
	SaltLength *int   `pulumi:"saltedLength,optional"`
}

func makeSalt(length int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, length)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func (*RandomSalt) Create(ctx context.Context, req infer.CreateRequest[RandomSaltArgs]) (infer.CreateResponse[RandomSaltState], error) {
	l := 4
	if req.Inputs.SaltLength != nil {
		l = *req.Inputs.SaltLength
	}
	salt := makeSalt(l)

	fmt.Printf("Running the create\n")

	return infer.CreateResponse[RandomSaltState]{
		ID: req.Name,
		Output: RandomSaltState{
			Salt:           salt,
			SaltedPassword: fmt.Sprintf("%s%s", salt, req.Inputs.Password),
			Password:       req.Inputs.Password,
			SaltLength:     req.Inputs.SaltLength,
		},
	}, nil
}

var _ = (infer.CustomUpdate[RandomSaltArgs, RandomSaltState])((*RandomSalt)(nil))

func (r *RandomSalt) Update(ctx context.Context, req infer.UpdateRequest[RandomSaltArgs, RandomSaltState]) (infer.UpdateResponse[RandomSaltState], error) {
	var redoSalt bool
	if req.State.SaltLength != nil && req.Inputs.SaltLength != nil {
		redoSalt = *req.State.SaltLength != *req.Inputs.SaltLength
	} else if req.State.SaltLength != nil || req.Inputs.SaltLength != nil {
		redoSalt = true
	}

	salt := req.State.Salt
	if redoSalt {
		if req.DryRun {
			return infer.UpdateResponse[RandomSaltState]{}, nil
		}
		l := 4
		if req.Inputs.SaltLength != nil {
			l = *req.Inputs.SaltLength
		}
		salt = makeSalt(l)
	}

	return infer.UpdateResponse[RandomSaltState]{
		Output: RandomSaltState{
			Salt:           salt,
			SaltedPassword: fmt.Sprintf("%s%s", salt, req.Inputs.Password),
			Password:       req.Inputs.Password,
			SaltLength:     req.Inputs.SaltLength,
		},
	}, nil
}

var _ = (infer.ExplicitDependencies[RandomSaltArgs, RandomSaltState])((*RandomSalt)(nil))

func (r *RandomSalt) WireDependencies(f infer.FieldSelector, args *RandomSaltArgs, state *RandomSaltState) {
	f.OutputField(&state.SaltedPassword).DependsOn(f.InputField(&args.Password), f.InputField(&args.SaltLength))
	f.OutputField(&state.Salt).DependsOn(f.InputField(&args.SaltLength))
}

type Config struct {
	Scream *bool `pulumi:"itsasecret,optional"`
}
