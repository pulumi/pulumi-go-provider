package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
)

const (
	fileContents = "the quick brown fox\n"
	// rememberFn is the token of the data source served after parameterization.
	rememberFn = "memory:index:remember"
)

// writeFile writes fileContents to a temporary file and returns its path.
func writeFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "memory.txt")
	require.NoError(t, os.WriteFile(path, []byte(fileContents), 0o600))
	return path
}

func TestParameterizeFromArgs(t *testing.T) {
	t.Parallel()
	prov := provider()

	resp, err := prov.Parameterize(t.Context(), p.ParameterizeRequest{
		Args: &p.ParameterizeRequestArgs{Args: []string{writeFile(t)}},
	})
	require.NoError(t, err)
	assert.Equal(t, p.ParameterizeResponse{
		Name:    "memory",
		Version: semver.MustParse("0.1.0"),
	}, resp)
}

func TestParameterizeRequiresSingleArg(t *testing.T) {
	t.Parallel()
	prov := provider()

	_, err := prov.Parameterize(t.Context(), p.ParameterizeRequest{
		Args: &p.ParameterizeRequestArgs{Args: []string{"a", "b"}},
	})
	assert.ErrorContains(t, err, "expected exactly one argument (the file path), got 2")
}

const expectedSchema = `{
  "name": "memory",
  "version": "0.1.0",
  "description": "A parameterized provider that remembers the contents of a file captured at parameterize time.",
  "config": {},
  "functions": {
    "memory:index:remember": {
      "description": "Return the contents of memory as captured when the provider was parameterized.",
      "inputs": {
        "type": "object"
      },
      "outputs": {
        "properties": {
          "contents": {
            "type": "string"
          }
        },
        "type": "object",
        "required": [
          "contents"
        ]
      }
    }
  },
  "parameterization": {
    "baseProvider": {
      "name": "remember",
      "version": "0.1.0"
    },
    "parameter": "dGhlIHF1aWNrIGJyb3duIGZveAo="
  }
}`

func TestSchema(t *testing.T) {
	t.Parallel()
	prov := provider()

	_, err := prov.Parameterize(t.Context(), p.ParameterizeRequest{
		Args: &p.ParameterizeRequestArgs{Args: []string{writeFile(t)}},
	})
	require.NoError(t, err)

	s, err := prov.GetSchema(t.Context(), p.GetSchemaRequest{})
	require.NoError(t, err)

	var blob json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(s.Schema), &blob))
	encoded, err := json.MarshalIndent(blob, "", "  ")
	require.NoError(t, err)
	assert.Equal(t, expectedSchema, string(encoded))
}

// TestSchemaParameterIsBase64 asserts that the file contents are embedded into the
// schema as a base64-encoded parameter.
func TestSchemaParameterIsBase64(t *testing.T) {
	t.Parallel()
	prov := provider()

	_, err := prov.Parameterize(t.Context(), p.ParameterizeRequest{
		Args: &p.ParameterizeRequestArgs{Args: []string{writeFile(t)}},
	})
	require.NoError(t, err)

	s, err := prov.GetSchema(t.Context(), p.GetSchemaRequest{})
	require.NoError(t, err)

	var raw struct {
		Parameterization struct {
			Parameter string `json:"parameter"`
		} `json:"parameterization"`
	}
	require.NoError(t, json.Unmarshal([]byte(s.Schema), &raw))
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte(fileContents)), raw.Parameterization.Parameter)
}

func TestInvoke(t *testing.T) {
	t.Parallel()
	prov := provider()

	_, err := prov.Parameterize(t.Context(), p.ParameterizeRequest{
		Args: &p.ParameterizeRequestArgs{Args: []string{writeFile(t)}},
	})
	require.NoError(t, err)

	r, err := prov.Invoke(t.Context(), p.InvokeRequest{
		Token: rememberFn,
	})
	require.NoError(t, err)
	assert.Empty(t, r.Failures)
	assert.Equal(t, fileContents, r.Return.Get("contents").AsString())
}
