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
	pulumi.ComponentResource

	login pulumi.StringOutput

	password pulumi.StringOutput

	passwordLength pulumi.IntInput
}

func (r *RandomLogin) Construct(name string, ctx *pulumi.Context) error {
	pet, err := random.NewRandomPet(ctx, name+"-pet", &random.RandomPetArgs{}, pulumi.Parent(r))
	if err != nil {
		return err
	}
	r.login = pet.ID().ToStringOutput()
	password, err := random.NewRandomPassword(ctx, name+"-password", &random.RandomPasswordArgs{
		Length: r.passwordLength,
	}, pulumi.Parent(r))
	if err != nil {
		return err
	}
	r.password = password.ID().ToStringOutput()

	return nil
}
