// Copyright 2026, Pulumi Corporation.
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
	"os/exec"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/stretchr/testify/require"
)

// TestIgnoreChangesLifecycle pins the engine-level contract for `ignoreChanges` against
// https://github.com/pulumi/pulumi-go-provider/issues/565.
//
// The test resource's live "members" output always drifts from the program's (absent)
// input. With `ignoreChanges: ["members"]`:
//
//   - the `--expect-no-changes` preview/update that ProgramTest runs after the initial
//     deployment must see no diff (no phantom `+ members` from substituting old outputs
//     for old inputs), and
//   - the step2 edit (slug change) triggers an Update, which fails inside the provider
//     if the ignored property arrives carrying the old output value.
func TestIgnoreChangesLifecycle(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "go", "build",
		"-o", "ignore_changes_consumer/pulumi-resource-test", "./ignore_changes_provider")
	require.NoError(t, cmd.Run(), strings.Join(cmd.Args, " "))

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: "ignore_changes_consumer",
		EditDirs: []integration.EditDir{{
			Dir:      "ignore_changes_consumer/step2",
			Additive: true,
		}},
	})
}
