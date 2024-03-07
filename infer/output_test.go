// Copyright 2023, Pulumi Corporation.  All rights reserved.

package infer

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer/internal/ende"
)

func TestOutputMapping(t *testing.T) {
	t.Parallel()
	p := func() resource.PropertyMap {
		return resource.PropertyMap{
			"sec":  resource.MakeSecret(resource.NewStringProperty("foo")),
			"unkn": resource.MakeComputed(resource.NewNullProperty()),
			"out": resource.NewOutputProperty(resource.Output{
				Known:  false,
				Secret: true,
			}),
			"plain": resource.NewStringProperty("known and public"),
		}
	}

	type decodeTarget struct {
		Sec   Output[string] `pulumi:"sec"`
		Unkn  Output[bool]   `pulumi:"unkn"`
		Out   Output[int]    `pulumi:"out"`
		Plain Output[string] `pulumi:"plain"`
	}

	target := decodeTarget{}
	enc, err := ende.Decode(p(), &target)
	require.NoError(t, err)

	assert.Falsef(t, target.Unkn.resolvable, "unknown properties serialize to unresolvable types")
	assert.True(t, target.Sec.resolvable, "secret properties serialize to knownable types")
	assert.True(t, target.Sec.resolved, "secret properties serialize to known types")
	assert.True(t, target.Sec.IsSecret(), "secret properties serialize to secret types")

	t.Run("derived outputs", func(t *testing.T) {
		unkn := Apply(target.Unkn, func(bool) string {
			assert.Fail(t, "Ran func on unknown value")
			return "FAILED"
		})
		assert.False(t, unkn.resolvable)

		kn := Apply(target.Sec, func(s string) bool { return s == "foo" })
		assert.True(t, kn.resolvable)

		combined := Apply2(target.Unkn, target.Sec, func(bool, string) int {
			assert.Fail(t, "Ran func on unknown value")
			return 0
		})
		assert.False(t, combined.resolvable)

		actual, err := ende.Encoder{}.Encode(struct {
			Unkn Output[string] `pulumi:"unkn"`
			Kn   Output[bool]   `pulumi:"kn"`
			Comb Output[int]    `pulumi:"comb"`
			Pub  Output[string] `pulumi:"pub"`
			Sec  Output[string] `pulumi:"asSec"`
		}{unkn, kn, combined, target.Sec.AsPublic(), target.Plain.AsSecret()})
		require.NoError(t, err)

		assert.Equal(t, resource.PropertyMap{
			"unkn": resource.MakeComputed(resource.NewNullProperty()),
			"kn":   resource.MakeSecret(resource.NewBoolProperty(true)),
			"comb": resource.NewOutputProperty(resource.Output{
				Secret: true,
				Known:  false,
			}),
			"pub":   resource.NewStringProperty("foo"),
			"asSec": resource.MakeSecret(resource.NewStringProperty("known and public")),
		}, actual)
	})

	actual, err := enc.Encode(target)
	require.NoError(t, err)

	assert.Equal(t, p(), actual)
}
