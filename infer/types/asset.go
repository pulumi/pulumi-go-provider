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

package types

import "github.com/pulumi/pulumi/sdk/v3/go/common/resource"

const AssetSignature = "a9e28acb8ab501f883219e7c9f624fb6"
const ArchiveSignature = "195f3948f6769324d4661e1e245f3a4d"

// AssetOrArchive is a union type that can represent either an Asset or an Archive.
// Setting both fields to non-nil values is an error.
// This type exists to accomomdate the semantics of the core Pulumi SDK's Asset type,
// which is also a union of Asset and Archive.
type AssetOrArchive struct {
	Asset   *resource.Asset   `pulumi:"a9e28acb8ab501f883219e7c9f624fb6,optional"`
	Archive *resource.Archive `pulumi:"195f3948f6769324d4661e1e245f3a4d,optional"`
}
