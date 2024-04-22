// This is intended to be an example of the enclosing SDK. Do not use this for
// cryptography.
package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	err := p.RunProvider("random-login", "0.1.0", provider())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}

func provider() p.Provider {
	return infer.Provider(infer.Options{
		Resources: []infer.InferredResource{infer.Resource[*RandomSalt, RandomSaltArgs, RandomSaltState]()},
		Components: []infer.InferredComponent{
			infer.Component[*RandomLogin, RandomLoginArgs, *RandomLoginOutput](),
			infer.Component[*MoreRandomPassword, MoreRandomPasswordArgs, *MoreRandomPasswordState](),
		},
		Config: infer.Config[Config](),
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{
			"random-login": "index",
		},
	})
}

// TODO: Deserialization does not yet work for external resources. Right now, it looks
// like this structure is only implementable in typescript, but that will need to change.
type MoreRandomPassword struct{}
type MoreRandomPasswordArgs struct {
	Length *random.RandomInteger `pulumi:"length" provider:"type=random@v4.8.1:index/randomInteger:RandomInteger"`
}

type MoreRandomPasswordState struct {
	pulumi.ResourceState
	Length   *random.RandomInteger  `pulumi:"length" provider:"type=random@v4.8.1:index/randomInteger:RandomInteger"`
	Password *random.RandomPassword `pulumi:"password" provider:"type=random@v4.8.1:index/randomPassword:RandomPassword"`
}

func (r *MoreRandomPassword) Construct(ctx *pulumi.Context, name, typ string, args MoreRandomPasswordArgs, opts pulumi.ResourceOption) (*MoreRandomPasswordState, error) {
	comp := &MoreRandomPasswordState{
		Length: args.Length,
	}
	err := ctx.RegisterComponentResource(typ, name, comp, opts)
	if err != nil {
		return nil, err
	}

	pArgs := &random.RandomPasswordArgs{
		Length: args.Length.Result,
	}

	config := infer.GetConfig[Config](ctx.Context())
	if config.Scream != nil {
		pArgs.Lower = pulumi.BoolPtr(*config.Scream)
	}

	comp.Password, err = random.NewRandomPassword(ctx, name+"-password", pArgs, pulumi.Parent(comp))
	if err != nil {
		return nil, err
	}
	return comp, nil
}

type RandomLogin struct{}
type RandomLoginArgs struct {
	PasswordLength pulumi.IntPtrInput `pulumi:"passwordLength"`
	PetName        bool               `pulumi:"petName"`
}

type RandomLoginOutput struct {
	pulumi.ResourceState
	PasswordLength pulumi.IntPtrInput `pulumi:"passwordLength"`
	PetName        bool               `pulumi:"petName"`
	// Outputs
	Username pulumi.StringOutput `pulumi:"username"`
	Password pulumi.StringOutput `pulumi:"password"`
}

func (r *RandomLogin) Construct(ctx *pulumi.Context, name, typ string, args RandomLoginArgs, opts pulumi.ResourceOption) (*RandomLoginOutput, error) {
	comp := &RandomLoginOutput{}
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
	var length pulumi.IntInput = pulumi.Int(16)
	if args.PasswordLength != nil {
		length = args.PasswordLength.ToIntPtrOutput().Elem()
	}
	password, err := random.NewRandomPassword(ctx, name+"-password", &random.RandomPasswordArgs{
		Length: length,
	}, pulumi.Parent(comp))
	if err != nil {
		return nil, err
	}
	comp.Password = password.Result

	return comp, nil
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
	rand.Seed(time.Now().UnixNano())
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func (*RandomSalt) Create(ctx context.Context, name string, input RandomSaltArgs, preview bool) (string, RandomSaltState, error) {
	l := 4
	if input.SaltLength != nil {
		l = *input.SaltLength
	}
	salt := makeSalt(l)

	fmt.Printf("Running the create")

	return name, RandomSaltState{
		Salt:           salt,
		SaltedPassword: fmt.Sprintf("%s%s", salt, input.Password),
		Password:       input.Password,
		SaltLength:     input.SaltLength,
	}, nil
}

var _ = (infer.CustomUpdate[RandomSaltArgs, RandomSaltState])((*RandomSalt)(nil))

func (r *RandomSalt) Update(ctx context.Context, id string, olds RandomSaltState, news RandomSaltArgs, preview bool) (RandomSaltState, error) {
	var redoSalt bool
	if olds.SaltLength != nil && news.SaltLength != nil {
		redoSalt = *olds.SaltLength != *news.SaltLength
	} else if olds.SaltLength != nil || news.SaltLength != nil {
		redoSalt = true
	}

	salt := olds.Salt
	if redoSalt {
		if preview {
			return RandomSaltState{}, nil
		}
		l := 4
		if news.SaltLength != nil {
			l = *news.SaltLength
		}
		salt = makeSalt(l)
	}

	return RandomSaltState{
		Salt:           salt,
		SaltedPassword: fmt.Sprintf("%s%s", salt, news.Password),
		Password:       news.Password,
		SaltLength:     news.SaltLength,
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
