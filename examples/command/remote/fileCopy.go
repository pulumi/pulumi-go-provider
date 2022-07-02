// Copyright 2016-2021, Pulumi Corporation.
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

package remote

import (
	"os"

	"github.com/pkg/sftp"

	r "github.com/pulumi/pulumi-go-provider/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

type CopyFile struct {
	// Input
	Connection Connection     `pulumi:"connection"`
	Triggers   *[]interface{} `pulumi:"triggers,optional"`
	LocalPath  string         `pulumi:"localPath"`
	RemotePath string         `pulumi:"remotePath"`
}

func (c *CopyFile) Annotate(a r.Annotator) {
	a.Describe(&c, "Copy a local file to a remote host.")

	a.Describe(&c.Connection, "The parameters with which to connect to the remote host.")
	a.Describe(&c.Triggers, "Trigger replacements on changes to this input.")
	a.Describe(&c.LocalPath, "The path of the file to be copied.")
	a.Describe(&c.RemotePath, "The destination path in the remote host.")
}

func (c *CopyFile) Create(ctx r.Context, name string, preview bool) (r.ID, error) {
	ctx.Log(diag.Debug, "Creating file: %s:%s from local file %s", c.Connection.Host, c.RemotePath, c.LocalPath)
	inner := func() error {
		src, err := os.Open(c.LocalPath)
		if err != nil {
			return err
		}
		defer src.Close()

		config, err := c.Connection.SShConfig()
		if err != nil {
			return err
		}
		client, err := c.Connection.Dial(ctx, config)
		if err != nil {
			return err
		}
		defer client.Close()

		sftp, err := sftp.NewClient(client)
		if err != nil {
			return err
		}
		defer sftp.Close()

		dst, err := sftp.Create(c.RemotePath)
		if err != nil {
			return err
		}

		_, err = dst.ReadFrom(src)
		return err
	}
	if err := inner(); err != nil {
		return "", err
	}
	return resource.NewUniqueHex("", 8, 0)

}

func (c *CopyFile) Delete(ctx r.Context, _ r.ID) error {
	ctx.Log(diag.Debug, "CopyFile delete is a no-op")
	return nil
}
