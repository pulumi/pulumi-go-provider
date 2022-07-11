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

package function

func New[F any](f F, description string) Function {
	return Function{
		F:           f,
		Description: description,
	}
}

type Function struct {
	// The underlying function
	F any
	// Description is the description of the function, if any.
	Description string
	// DeprecationMessage indicates whether or not the function is deprecated.
	DeprecationMessage string
}
