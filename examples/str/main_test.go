// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"encoding/json"
	"testing"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	integration "github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const schema = `{
  "name": "str",
  "version": "0.1.0",
  "config": {},
  "provider": {},
  "functions": {
    "str:index:giveMeAString": {
      "description": "Return a string, withing any inputs",
      "inputs": {
        "type": "object"
      },
      "outputs": {
        "properties": {
          "out": {
            "type": "string"
          }
        },
        "type": "object",
        "required": [
          "out"
        ]
      }
    },
    "str:index:print": {
      "description": "Print to stdout",
      "inputs": {
        "properties": {
          "s": {
            "type": "string"
          }
        },
        "type": "object",
        "required": [
          "s"
        ]
      },
      "outputs": {
        "type": "object"
      }
    },
    "str:index:replace": {
      "description": "Replace returns a copy of the string s with all\nnon-overlapping instances of old replaced by new.\nIf old is empty, it matches at the beginning of the string\nand after each UTF-8 sequence, yielding up to k+1 replacements\nfor a k-rune string.",
      "inputs": {
        "properties": {
          "new": {
            "type": "string",
            "description": "The string to replace ` + "`Old`" + ` with."
          },
          "old": {
            "type": "string",
            "description": "The string to replace."
          },
          "s": {
            "type": "string",
            "description": "The string where the replacement takes place."
          }
        },
        "type": "object",
        "required": [
          "s",
          "old",
          "new"
        ]
      },
      "outputs": {
        "properties": {
          "out": {
            "type": "string"
          }
        },
        "type": "object",
        "required": [
          "out"
        ]
      }
    },
    "str:regex:replace": {
      "description": "Replace returns a copy of ` + "`s`" + `, replacing matches of the ` + "`old`" + `\nwith the replacement string ` + "`new`" + `.",
      "inputs": {
        "properties": {
          "new": {
            "type": "string"
          },
          "pattern": {
            "type": "string"
          },
          "s": {
            "type": "string"
          }
        },
        "type": "object",
        "required": [
          "s",
          "pattern",
          "new"
        ]
      },
      "outputs": {
        "properties": {
          "out": {
            "type": "string"
          }
        },
        "type": "object",
        "required": [
          "out"
        ]
      }
    }
  }
}`

func TestSchema(t *testing.T) {
	server, err := integration.NewServer(t.Context(),
		"str",
		semver.Version{Minor: 1},
		integration.WithProvider(provider()),
	)
	require.NoError(t, err)

	s, err := server.GetSchema(p.GetSchemaRequest{})
	assert.NoError(t, err)
	blob := json.RawMessage{}
	err = json.Unmarshal([]byte(s.Schema), &blob)
	assert.NoError(t, err)
	encoded, err := json.MarshalIndent(blob, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, schema, string(encoded))
}

func TestInvokes(t *testing.T) {
	server, err := integration.NewServer(t.Context(),
		"str",
		semver.Version{Minor: 1},
		integration.WithProvider(provider()),
	)
	require.NoError(t, err)

	r, err := server.Invoke(p.InvokeRequest{
		Token: "str:index:replace",
		Args: property.NewMap(map[string]property.Value{
			"s":   property.New("foo!bar"),
			"old": property.New("!"),
			"new": property.New("-"),
		}),
	})
	assert.NoError(t, err)
	assert.Empty(t, r.Failures)
	assert.Equal(t, "foo-bar", r.Return.Get("out").AsString())

	r, err = server.Invoke(p.InvokeRequest{
		Token: "str:regex:replace",
		Args: property.NewMap(map[string]property.Value{
			"s":       property.New("fizz, buzz, zzz..."),
			"pattern": property.New("z+"),
			"new":     property.New("Z"),
		}),
	})
	assert.NoError(t, err)
	assert.Empty(t, r.Failures)
	assert.Equal(t, "fiZ, buZ, Z...", r.Return.Get("out").AsString())
}
