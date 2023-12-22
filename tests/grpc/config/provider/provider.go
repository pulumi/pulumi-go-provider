// Copyright 2023, Pulumi Corporation.
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

package config

import (
	"encoding/json"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func Provider() p.Provider {
	return infer.Provider(infer.Options{
		Resources: []infer.InferredResource{infer.Resource[*Get, GetArgs, GetState]()},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{
			"provider": "index",
		},
		Config: infer.Config[*Config](),
	})
}

type Config struct {
	String    string            `pulumi:"s"`
	Bool      bool              `pulumi:"b"`
	Int       int               `pulumi:"i"`
	Map       map[string]string `pulumi:"m"`
	Arr       []string          `pulumi:"a"`
	Nested    ConfigNested      `pulumi:"n"`
	ArrNested []ConfigNested    `pulumi:"an"`

	DString string `pulumi:"ds,optional"`
	DBool   *bool  `pulumi:"db,optional"`
	DInt    int    `pulumi:"di,optional"`
}

type ConfigNested struct {
	String string            `pulumi:"s"`
	Bool   bool              `pulumi:"b"`
	Int    int               `pulumi:"i"`
	Map    map[string]string `pulumi:"m"`
	Arr    []string          `pulumi:"a"`
}

var _ = (infer.Annotated)((*Config)(nil))

func (c *Config) Annotate(a infer.Annotator) {
	a.SetDefault(&c.String, nil, "STRING")
	a.SetDefault(&c.Bool, nil, "BOOL")
	a.SetDefault(&c.Int, nil, "INT")

	a.SetDefault(&c.DString, "defString")
	a.SetDefault(&c.DBool, true)
	a.SetDefault(&c.DInt, 42)
}

type Get struct{}
type GetArgs struct{}
type GetState struct {
	Config string `pulumi:"config"`
}

func (*Get) Create(ctx p.Context, name string, input GetArgs, preview bool) (string, GetState, error) {
	config := infer.GetConfig[Config](ctx)
	bytes, err := json.Marshal(&config)
	return name, GetState{Config: string(bytes)}, err
}
