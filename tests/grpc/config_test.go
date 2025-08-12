// Copyright 2022, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package grpc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	replay "github.com/pulumi/providertest/replay"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	config "github.com/pulumi/pulumi-go-provider/tests/grpc/config/provider"
)

// injectFrameworkVersion replaces the {{VERSION}} placeholder in the log with the
// version of the provider obtained from the repository's root .version file.
func injectFrameworkVersion(logTemplate string) string {
	// Get the absolute path to the .version file.
	path, err := filepath.Abs("../../.version")
	if err != nil {
		panic(fmt.Errorf("unable to get absolute path: %w", err))
	}

	// Read the contents of the .version file.
	version, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("unable to read .version file: %w", err))
	}

	// Replace the placeholder "{{VERSION}}" with the actual version.
	return strings.ReplaceAll(logTemplate, "{{VERSION}}", string(version))
}

// These inputs were created by running `pulumi up` with PULUMI_DEBUG_GRPC=logs.json in
// ./config/consumer.

// TestBasicConfig checks that we can handle deserializing basic configuration values.
//
// These test values were derived from a Pulumi YAML program, which does not JSON encode
// it's values.
func TestBasicConfig(t *testing.T) {
	sequence := injectFrameworkVersion(`[
  {
    "method": "/pulumirpc.ResourceProvider/CheckConfig",
    "request": {
      "urn": "urn:pulumi:dev::consume-random-login::pulumi:providers:config::provider",
      "olds": {
        "a": [
          "one",
          "two"
        ],
        "b": true,
        "db": true,
        "di": 42,
        "ds": "defString",
        "i": 42,
        "m": {
          "fizz": "buzz"
        },
        "n": {
          "a": [
            "one",
            "two"
          ],
          "b": true,
          "i": 42,
          "m": {
            "fizz": "buzz"
          },
          "s": "foo"
        },
        "s": "foo"
      },
      "news": {
        "a": [
          "one",
          "two"
        ],
        "b": true,
        "i": 42,
        "m": {
          "fizz": "buzz"
        },
        "n": {
          "a": [
            "one",
            "two"
          ],
          "b": true,
          "i": 42,
          "m": {
            "fizz": "buzz"
          },
          "s": "foo"
        },
        "s": "foo"
      }
    },
    "response": {
      "inputs": {
        "__internal": {
          "pulumi-go-provider-infer": true,
          "pulumi-go-provider-version": "{{VERSION}}"
        },
        "a": [
          "one",
          "two"
        ],
        "b": true,
        "db": true,
        "di": 42,
        "ds": "defString",
        "i": 42,
        "m": {
          "fizz": "buzz"
        },
        "n": {
          "a": [
            "one",
            "two"
          ],
          "b": true,
          "i": 42,
          "m": {
            "fizz": "buzz"
          },
          "s": "foo"
        },
        "s": "foo",
        "version": "1.0.0"
      }
    },
    "metadata": {
      "kind": "resource",
      "mode": "client",
      "name": "config"
    }
  },
  {
    "method": "/pulumirpc.ResourceProvider/DiffConfig",
    "request": {
      "urn": "urn:pulumi:dev::consume-random-login::pulumi:providers:config::provider",
      "olds": {
        "a": [
          "one",
          "two"
        ],
        "b": true,
        "db": true,
        "di": 42,
        "ds": "defString",
        "i": 42,
        "m": {
          "fizz": "buzz"
        },
        "n": {
          "a": [
            "one",
            "two"
          ],
          "b": true,
          "i": 42,
          "m": {
            "fizz": "buzz"
          },
          "s": "foo"
        },
        "s": "foo"
      },
      "news": {
        "a": [
          "one",
          "two"
        ],
        "b": true,
        "db": true,
        "di": 42,
        "ds": "defString",
        "i": 42,
        "m": {
          "fizz": "buzz"
        },
        "n": {
          "a": [
            "one",
            "two"
          ],
          "b": true,
          "i": 42,
          "m": {
            "fizz": "buzz"
          },
          "s": "foo"
        },
        "s": "foo"
      }
    },
    "response": {
      "changes": "DIFF_NONE",
      "hasDetailedDiff": true
    },
    "metadata": {
      "kind": "resource",
      "mode": "client",
      "name": "config"
    }
  },
  {
    "method": "/pulumirpc.ResourceProvider/Configure",
    "request": {
      "variables": {
        "config:config:a": "[\"one\",\"two\"]",
        "config:config:b": "true",
        "config:config:db": "true",
        "config:config:di": "42",
        "config:config:ds": "defString",
        "config:config:i": "42",
        "config:config:m": "{\"fizz\":\"buzz\"}",
        "config:config:n": "{\"a\":[\"one\",\"two\"],\"b\":true,\"i\":42,\"m\":{\"fizz\":\"buzz\"},\"s\":\"foo\"}",
        "config:config:s": "foo"
      },
      "args": {
        "a": [
          "one",
          "two"
        ],
        "b": true,
        "db": true,
        "di": 42,
        "ds": "defString",
        "i": 42,
        "m": {
          "fizz": "buzz"
        },
        "n": {
          "a": [
            "one",
            "two"
          ],
          "b": true,
          "i": 42,
          "m": {
            "fizz": "buzz"
          },
          "s": "foo"
        },
        "s": "foo"
      },
      "acceptSecrets": true,
      "acceptResources": true
    },
    "response": {
      "acceptSecrets": true,
      "supportsPreview": true,
      "acceptResources": true,
      "acceptOutputs": true
    },
    "metadata": {
      "kind": "resource",
      "mode": "client",
      "name": "config"
    }
  }
]`)
	replayConfig(t, sequence)
}

func TestConfigWithSecrets(t *testing.T) {
	sequence := injectFrameworkVersion(`[
  {
    "method": "/pulumirpc.ResourceProvider/CheckConfig",
    "request": {
      "urn": "urn:pulumi:dev::consume-random-login::pulumi:providers:config::provider",
      "olds": {
        "a": [
          "one",
          "two"
        ],
        "b": true,
        "db": true,
        "di": 42,
        "ds": "defString",
        "i": 42,
        "m": {
          "fizz": "buzz"
        },
        "n": {
          "a": [
            "one",
            "two"
          ],
          "b": true,
          "i": 42,
          "m": {
            "fizz": "buzz"
          },
          "s": "foo"
        },
        "s": "foo"
      },
      "news": {
        "a": [
          "one",
          "two"
        ],
        "b": true,
        "i": 42,
        "m": {
          "fizz": "buzz"
        },
        "n": {
          "a": [
            "one",
            "two"
          ],
          "b": true,
          "i": 42,
          "m": {
            "fizz": "buzz"
          },
          "s": "foo"
        },
        "s": "foo"
      }
    },
    "response": {
      "inputs": {
        "__internal": {
          "pulumi-go-provider-infer": true,
          "pulumi-go-provider-version": "{{VERSION}}"
        },
        "a": [
          "one",
          "two"
        ],
        "b": true,
        "db": true,
        "di": 42,
        "ds": "defString",
        "i": 42,
        "m": {
          "fizz": "buzz"
        },
        "n": {
          "a": [
            "one",
            "two"
          ],
          "b": true,
          "i": 42,
          "m": {
            "fizz": "buzz"
          },
          "s": "foo"
        },
        "s": "foo",
        "version": "1.0.0"
      }
    },
    "metadata": {
      "kind": "resource",
      "mode": "client",
      "name": "config"
    }
  },
  {
    "method": "/pulumirpc.ResourceProvider/DiffConfig",
    "request": {
      "urn": "urn:pulumi:dev::consume-random-login::pulumi:providers:config::provider",
      "olds": {
        "a": [
          "one",
          "two"
        ],
        "b": true,
        "db": true,
        "di": 42,
        "ds": "defString",
        "i": 42,
        "m": {
          "fizz": "buzz"
        },
        "n": {
          "a": [
            "one",
            "two"
          ],
          "b": true,
          "i": 42,
          "m": {
            "fizz": "buzz"
          },
          "s": "foo"
        },
        "s": "foo"
      },
      "news": {
        "a": [
          "one",
          "two"
        ],
        "b": true,
        "db": true,
        "di": 42,
        "ds": "defString",
        "i": 42,
        "m": {
          "fizz": "buzz"
        },
        "n": {
          "a": [
            "one",
            "two"
          ],
          "b": true,
          "i": 42,
          "m": {
            "fizz": "buzz"
          },
          "s": "foo"
        },
        "s": "foo"
      }
    },
    "response": {
      "changes": "DIFF_NONE",
      "hasDetailedDiff": true
    },
    "metadata": {
      "kind": "resource",
      "mode": "client",
      "name": "config"
    }
  },
  {
    "method": "/pulumirpc.ResourceProvider/Configure",
    "request": {
      "variables": {
        "config:config:a": "[\"one\",\"two\"]",
        "config:config:b": "true",
        "config:config:db": "true",
        "config:config:di": "42",
        "config:config:ds": "defString",
        "config:config:i": "42",
        "config:config:m": "{\"fizz\":\"buzz\"}",
        "config:config:n": "{\"a\":[\"one\",\"two\"],\"b\":true,\"i\":42,\"m\":{\"fizz\":\"buzz\"},\"s\":\"foo\"}",
        "config:config:s": "foo"
      },
      "args": {
        "a": [
          "one",
          "two"
        ],
        "b": true,
        "db": true,
        "di": 42,
        "ds": "defString",
        "i": 42,
        "m": {
          "fizz": "buzz"
        },
        "n": {
          "a": [
            "one",
            "two"
          ],
          "b": true,
          "i": 42,
          "m": {
            "fizz": "buzz"
          },
          "s": "foo"
        },
        "s": "foo"
      },
      "acceptSecrets": true,
      "acceptResources": true
    },
    "response": {
      "acceptSecrets": true,
      "supportsPreview": true,
      "acceptResources": true,
      "acceptOutputs": true
    },
    "metadata": {
      "kind": "resource",
      "mode": "client",
      "name": "config"
    }
  }
]`)
	replayConfig(t, sequence)
}

// TestJSONEncodedConfig shows that we can correctly interpret JSON encoded config values.
//
// These test values were derived from the following TypeScript program. TypeScript does
// JSON encode it's values:
//
//	import * as pulumi from "@pulumi/pulumi";
//	import * as p from "config";
//
//	export const c = new p.Provider("ts", {
//	  s: pulumi.secret("foo"),
//	  b: true,
//	  i: 42,
//	  m: {    fizz: "buzz",  },
//	  a: [
//	    pulumi.secret("one"),
//	    "two",
//	  ],
//	  n: {
//	    s: pulumi.secret("foo"),
//	    b: true,
//	    i: 42,
//	    m: {    fizz: "buzz",  },
//	    a: [
//	      "one",
//	      "two",
//	    ],
//	  },
//	});
//
//	export const config = new p.Get("get", {}, {provider: c}).config;
//
// To re-generate these values, build the `config` test provider:
//
//	cd tests/grpc/config && go build -o pulumi-resource-config .
//
// Then create a Pulumi project, specify the provider path to the binary:
//
//	plugins:
//	  providers:
//	    - name: config
//	      path: ..
//
// Write the program and run `consumer/run.sh` to generate gRPC logs.
func TestJSONEncodedConfig(t *testing.T) {
	replayConfig(t, injectFrameworkVersion(`[{
    "method": "/pulumirpc.ResourceProvider/CheckConfig",
    "request": {
        "urn": "urn:pulumi:test::test::pulumi:providers:config::ts",
        "olds": {},
        "news": {
            "a": "[\"one\",\"two\"]",
            "b": "true",
            "db": "true",
            "di": "42",
            "ds": "defString",
            "i": "42",
            "m": "{\"fizz\":\"buzz\"}",
            "n": "{\"s\":\"foo\",\"b\":true,\"i\":42,\"m\":{\"fizz\":\"buzz\"},\"a\":[\"one\",\"two\"]}",
            "s": "foo",
            "version": "0.0.1"
        }
    },
    "response": {
        "inputs": {
            "__internal": {
              "pulumi-go-provider-infer": true,
              "pulumi-go-provider-version": "{{VERSION}}"
            },
            "a": [
                "one",
                "two"
            ],
            "b": true,
            "db": true,
            "di": 42,
            "ds": "defString",
            "i": 42,
            "m": {
                "fizz": "buzz"
            },
            "n": {
                "a": [
                    "one",
                    "two"
                ],
                "b": true,
                "i": 42,
                "m": {
                    "fizz": "buzz"
                },
                "s": "foo"
            },
            "s": "foo",
            "version": "1.0.0"
        }
    },
    "metadata": {
        "kind": "resource",
        "mode": "client",
        "name": "config"
    }
}]`))
}

func replayConfig(t *testing.T, jsonLog string) {
	s, err := p.RawServer("config", "1.0.0", config.Provider())(nil)
	require.NoError(t, err)
	replay.ReplaySequence(t, s, jsonLog)
}
