package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	r "github.com/pulumi/pulumi-go-provider/resource"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	err := p.RunProvider("random-login", semver.Version{Minor: 1},
		infer.NewProvider().
			WithResources(infer.Resource[*RandomSalt, RandomSaltArgs, RandomSaltState]()).
			WithComponents(infer.Component[*RandomLogin, RandomLoginArgs, *RandomLoginOutput]()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}

type RandomLogin struct{}
type RandomLoginArgs struct {
	PasswordLength pulumi.IntPtrInput `pulumi:"passwordLength,optional"`
}
type RandomLoginOutput struct {
	pulumi.ResourceState
	PasswordLength pulumi.IntPtrInput `pulumi:"passwordLength,optional"`
	// Outputs
	Username pulumi.StringOutput `pulumi:"username"`
	Password pulumi.StringOutput `pulumi:"password"`
}

func (r *RandomLogin) Construct(ctx *pulumi.Context, name, typ string, inputs RandomLoginArgs, opts pulumi.ResourceOption) (*RandomLoginOutput, error) {
	comp := &RandomLoginOutput{}
	err := ctx.RegisterComponentResource(typ, name, comp, opts)
	if err != nil {
		return nil, err
	}
	pet, err := random.NewRandomPet(ctx, name+"-pet", &random.RandomPetArgs{}, pulumi.Parent(comp))
	if err != nil {
		return nil, err
	}
	comp.Username = pet.ID().ToStringOutput()
	var length pulumi.IntInput = pulumi.Int(16)
	if inputs.PasswordLength != nil {
		length = inputs.PasswordLength.ToIntPtrOutput().Elem()
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

func (*RandomSalt) Create(ctx p.Context, name string, input RandomSaltArgs, preview bool) (string, RandomSaltState, error) {
	l := 4
	if input.SaltLength != nil {
		l = *input.SaltLength
	}
	salt := makeSalt(l)

	return name, RandomSaltState{
		Salt:           salt,
		SaltedPassword: fmt.Sprintf("%s%s", salt, input.Password),
		Password:       input.Password,
		SaltLength:     input.SaltLength,
	}, nil
}

func (r *RandomSalt) Delete(ctx r.Context, id string) error {
	// We don't manage external state, so just do nothing
	return nil
}

var _ = (infer.CustomUpdate[RandomSaltArgs, RandomSaltState])((*RandomSalt)(nil))

func (r *RandomSalt) Update(ctx p.Context, id string, olds RandomSaltState, news RandomSaltArgs, preview bool) (RandomSaltState, error) {
	var redoSalt bool
	if olds.SaltLength != nil && news.SaltLength != nil {
		redoSalt = *olds.SaltLength != *news.SaltLength
	} else if olds.SaltLength != nil || news.SaltLength != nil {
		redoSalt = true
	}

	salt := olds.Salt
	if redoSalt {
		// ctx.MarkComputed(&r.Salt)
		// ctx.MarkComputed(&r.SaltedPassword)
		if preview {
			return RandomSaltState{}, nil
		}
		l := 4
		if news.SaltLength != nil {
			l = *news.SaltLength
		}
		salt = makeSalt(l)
	}
	if olds.Password != news.Password {
		// ctx.MarkComputed(&r.SaltedPassword)
	}

	return RandomSaltState{
		Salt:           salt,
		SaltedPassword: fmt.Sprintf("%s%s", salt, news.Password),
		Password:       news.Password,
		SaltLength:     news.SaltLength,
	}, nil
}
