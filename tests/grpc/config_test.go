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
	"testing"

	replay "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	config "github.com/pulumi/pulumi-go-provider/tests/grpc/config/provider"
)

// These inputs were created by running `pulumi up` with PULUMI_DEBUG_GRPC=logs.json in
// ./config/consumer.

func TestBasicConfig(t *testing.T) {
	sequence := `[
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
]`
	replayConfig(t, sequence)
}

func TestConfigWithSecrets(t *testing.T) {
	sequence := `[
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
]`
	replayConfig(t, sequence)
}

func replayConfig(t *testing.T, jsonLog string) {
	s, err := p.RawServer("config", "1.0.0", config.Provider())
	require.NoError(t, err)
	replay.ReplaySequence(t, s, jsonLog)
}
