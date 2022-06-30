package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/blang/semver"
	provider "github.com/pulumi/pulumi-go-provider"
	r "github.com/pulumi/pulumi-go-provider/resource"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	err := provider.Run("random-login", semver.Version{Minor: 1},
		provider.Components(&RandomLogin{}),
		provider.Resources(&RandomSalt{}),
		provider.PartialSpec(schema.PackageSpec{}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}

type RandomLogin struct {
	pulumi.ResourceState

	// Outputs
	Username pulumi.StringOutput `pulumi:"username" provider:"output"`
	Password pulumi.StringOutput `pulumi:"password" provider:"output"`

	// Inputs
	PasswordLength pulumi.IntPtrInput `pulumi:"passwordLength"`
}

func (r *RandomLogin) Construct(name string, ctx *pulumi.Context) error {
	pet, err := random.NewRandomPet(ctx, name+"-pet", &random.RandomPetArgs{}, pulumi.Parent(r))
	if err != nil {
		return err
	}
	r.Username = pet.ID().ToStringOutput()
	var length pulumi.IntInput = pulumi.Int(16)
	if r.PasswordLength != nil {
		length = r.PasswordLength.ToIntPtrOutput().Elem()
	}
	password, err := random.NewRandomPassword(ctx, name+"-password", &random.RandomPasswordArgs{
		Length: length,
	}, pulumi.Parent(r))
	if err != nil {
		return err
	}
	r.Password = password.Result.ToStringOutput()

	return nil
}

type RandomSalt struct {

	// Outputs
	Salt           string `pulumi:"salt" provider:"output"`
	SaltedPassword string `pulumi:"saltedPassword" provider:"output"`

	// Inputs
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

func (r *RandomSalt) Create(ctx r.Context, name string, preview bool) (string, error) {
	l := 4
	if r.SaltLength != nil {
		l = *r.SaltLength
	}
	r.Salt = makeSalt(l)

	r.SaltedPassword = fmt.Sprintf("%s%s", r.Salt, r.Password)

	return name, nil
}

func (r *RandomSalt) Delete(ctx r.Context, id string) error {
	// We don't manage external state, so just do nothing
	return nil
}

var _ = (r.Update)((*RandomSalt)(nil))

func (r *RandomSalt) Update(ctx r.Context, id string, newSalt any, ignoreChanges []string, preview bool) error {
	new := newSalt.(*RandomSalt)
	var redoSalt bool
	if r.SaltLength != nil && new.SaltLength != nil {
		redoSalt = *r.SaltLength != *new.SaltLength
	} else if r.SaltLength != nil || new.SaltLength != nil {
		redoSalt = true
	}
	r.SaltLength = new.SaltLength

	if redoSalt {
		ctx.MarkComputed(&r.Salt)
		ctx.MarkComputed(&r.SaltedPassword)
		return nil
		l := 4
		if r.SaltLength != nil {
			l = *r.SaltLength
		}
		r.Salt = makeSalt(l)
	}
	if r.Password != new.Password {
		ctx.MarkComputed(&r.SaltedPassword)
	}
	r.Password = new.Password

	r.SaltedPassword = fmt.Sprintf("%s%s", r.Salt, r.Password)
	return nil
}
