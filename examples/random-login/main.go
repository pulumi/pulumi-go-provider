package main

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/blang/semver"
	provider "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	provider.Run("random-login", semver.Version{Minor: 1},
		provider.Components(&RandomLogin{}),
		provider.Resources(&RandomSalt{}))
}

type RandomLogin struct {
	pulumi.ResourceState

	// Outputs
	Username pulumi.StringOutput `pulumi:"username"`
	Password pulumi.StringOutput `pulumi:"password"`

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
	Salt           string `pulumi:"salt"`
	SaltedPassword string `pulumi:"saltedPassword"`

	// Inputs
	Password   string `pulumi:"password"`
	SaltLength *int   `pulumi:"saltedLength,optional"`
}

func (r *RandomSalt) Create(ctx context.Context, name string, preview bool) (string, error) {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	l := 4
	if r.SaltLength != nil {
		l = *r.SaltLength
	}

	b := make([]rune, l)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	r.Salt = string(b)

	r.SaltedPassword = fmt.Sprintf("%s%s", r.Salt, r.Password)

	return name, nil
}

func (r *RandomSalt) Delete(ctx context.Context, id string) error {
	// We don't manage external state, so just do nothing
	return nil
}
