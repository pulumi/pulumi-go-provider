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

package openapi

import "encoding/json"

type encoding struct {
	marshal   func(any) ([]byte, error)
	unmarshal func([]byte, any) error
}

type encoder map[string]encoding

func (e encoder) get(mime string) (encoding, bool) {
	enc, ok := e[mime]
	return enc, ok
}

func (e encoder) has(mime string) bool {
	_, ok := e.get(mime)
	return ok
}

var serde = encoder{
	"application/json": encoding{
		marshal:   json.Marshal,
		unmarshal: json.Unmarshal,
	},
}
