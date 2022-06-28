package main

import (
	"github.com/blang/semver"
	provider "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	provider.Run("random-login", semver.Version{Minor: 1}, provider.Components(
		&RandomLogin{},
	))
}

type RandomLogin struct {
	pulumi.ResourceState

	// Outputs
	Login    pulumi.StringOutput `pulumi:"login"`
	Password pulumi.StringOutput `pulumi:"password"`

	// Inputs
	PasswordLength pulumi.IntPtrInput `pulumi:"passwordLength"`
}

func (r *RandomLogin) Construct(name string, ctx *pulumi.Context) error {
	pet, err := random.NewRandomPet(ctx, name+"-pet", &random.RandomPetArgs{}, pulumi.Parent(r))
	if err != nil {
		return err
	}
	r.Login = pet.ID().ToStringOutput()
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
