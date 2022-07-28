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
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/pulumi/pulumi-go-provider/examples/command/util"
	r "github.com/pulumi/pulumi-go-provider/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/retry"
	"golang.org/x/crypto/ssh"
)

type Connection struct {
	User       string  `pulumi:"user,optional"`
	Password   *string `pulumi:"password,optional"`
	Host       string  `pulumi:"host"`
	Port       int     `pulumi:"port,optional"`
	PrivateKey *string `pulumi:"privateKey,optional"`
}

func (con *Connection) Annotate(a r.Annotator) {
	a.Describe(&con, "A local command to be executed.\n"+
		"This command can be inserted into the life cycles of other resources using the\n"+
		"`dependsOn` or `parent` resource options. A command is considered to have\n"+
		"failed when it finished with a non-zero exit code. This will fail the CRUD step\n"+
		"of the `Command` resource.")

	a.Describe(&con.User, "The user that we should use for the connection.")
	a.SetDefault(&con.User, "root")

	a.Describe(&con.Password, "The password we should use for the connection.")
	a.Describe(&con.Host, "The address of the resource to connect to.")

	a.Describe(&con.Port, "The port to connect to.")
	a.SetDefault(&con.Port, 22)

	a.Describe(&con.PrivateKey, "The contents of an SSH key to use for the connection. This takes preference over the password if provided.")
}

// Generate an ssh config from a connection specification.
func (con Connection) SShConfig() (*ssh.ClientConfig, error) {
	config := &ssh.ClientConfig{
		User:            con.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if con.PrivateKey != nil {
		signer, err := ssh.ParsePrivateKey([]byte(*con.PrivateKey))
		if err != nil {
			return nil, err
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	}
	if con.Password != nil {
		config.Auth = append(config.Auth, ssh.Password(*con.Password))
		config.Auth = append(config.Auth, ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) (answers []string, err error) {
			for i := range questions {
				answers[i] = *con.Password
			}
			return answers, err
		}))
	}

	return config, nil
}

// Dial a ssh client connection from a ssh client configuration, retrying as necessary.
func (con Connection) Dial(ctx context.Context, config *ssh.ClientConfig) (*ssh.Client, error) {
	var client *ssh.Client
	var err error
	_, _, err = retry.Until(ctx, retry.Acceptor{
		Accept: func(try int, nextRetryTime time.Duration) (bool, interface{}, error) {
			client, err = ssh.Dial("tcp",
				net.JoinHostPort(con.Host, fmt.Sprintf("%d", con.Port)),
				config)
			if err != nil {
				if try > 10 {
					return true, nil, err
				}
				return false, nil, nil
			}
			return true, nil, nil
		},
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

var _ r.Update = (*Command)(nil)

type Command struct {
	// Input
	Connection  Connection         `pulumi:"connection"`
	Interpreter *[]string          `pulumi:"interpreter,optional"`
	Dir         *string            `pulumi:"dir,optional"`
	Environment *map[string]string `pulumi:"environment,optional"`
	Triggers    *[]interface{}     `pulumi:"triggers,optional"`
	Create_     string             `pulumi:"create"`
	Delete_     *string            `pulumi:"delete,optional"`
	Update_     *string            `pulumi:"update,optional"`
	Stdin       *string            `pulumi:"stdin,optional"`

	// Output
	Stdout string `pulumi:"stdout" provider:"output"`
	Stderr string `pulumi:"stderr" provider:"output"`
}

func (c *Command) Annotate(a r.Annotator) {
	a.Describe(&c, "A command to run on a remote host.\nThe connection is established via ssh.")
	a.Describe(&c.Connection, "The parameters with which to connect to the remote host")
	a.Describe(&c.Environment, "Additional environment variables available to the command's process.")
	a.Describe(&c.Triggers, "Trigger replacements on changes to this input.")
	a.Describe(&c.Create_, "The command to run on create.")
	a.Describe(&c.Delete_, "The command to run on delete.")
	a.Describe(&c.Update_, "The command to run on update, if empty, create will run again.")
	a.Describe(&c.Stdin, "Pass a string to the command's process as standard in")

	a.Describe(&c.Stdout, "The standard output of the command's process")
	a.Describe(&c.Stderr, "")
}

// Create executes the create command, sets Stdout and Stderr, and returns a unique
// ID for the command execution
func (c *Command) Create(ctx r.Context, name string, preview bool) (r.ID, error) {
	stdout, stderr, id, err := c.run(ctx, c.Create_)
	c.Stdout = stdout
	c.Stderr = stderr
	return id, err
}

// RunDelete executes the delete command
func (c *Command) Delete(ctx r.Context, _ r.ID) error {
	if c.Delete_ == nil {
		return nil
	}
	_, _, _, err := c.run(ctx, *c.Delete_)
	return err
}

func (c *Command) Update(ctx r.Context, id r.ID, newC any, _ []string, preview bool) error {
	if preview {
		ctx.MarkComputed(&c.Stderr)
		ctx.MarkComputed(&c.Stdout)
		return nil
	}
	new := newC.(*Command)
	c.Update_ = new.Update_
	if c.Update_ != nil {
		stdout, stderr, _, err := c.run(ctx, *c.Update_)
		c.Stdout = stdout
		c.Stderr = stderr
		return err
	}
	stdout, stderr, id, err := c.run(ctx, c.Create_)
	c.Stdout = stdout
	c.Stderr = stderr
	return err
}

func (c *Command) run(ctx r.Context, cmd string) (string, string, string, error) {
	config, err := c.Connection.SShConfig()
	if err != nil {
		return "", "", "", err
	}

	client, err := c.Connection.Dial(ctx, config)
	if err != nil {
		return "", "", "", err
	}

	session, err := client.NewSession()
	if err != nil {
		return "", "", "", err
	}
	defer session.Close()

	if c.Environment != nil {
		for k, v := range *c.Environment {
			session.Setenv(k, v)
		}
	}

	if c.Stdin != nil && len(*c.Stdin) > 0 {
		session.Stdin = strings.NewReader(*c.Stdin)
	}

	id, err := resource.NewUniqueHex("", 8, 0)
	if err != nil {
		return "", "", "", err
	}

	stdoutr, stdoutw, err := os.Pipe()
	if err != nil {
		return "", "", "", err
	}
	stderrr, stderrw, err := os.Pipe()
	if err != nil {
		return "", "", "", err
	}
	session.Stdout = stdoutw
	session.Stderr = stderrw

	var stdoutbuf bytes.Buffer
	var stderrbuf bytes.Buffer

	stdouttee := io.TeeReader(stdoutr, &stdoutbuf)
	stderrtee := io.TeeReader(stderrr, &stderrbuf)

	stdoutch := make(chan struct{})
	stderrch := make(chan struct{})
	go util.CopyOutput(ctx, stdouttee, stdoutch, diag.Debug)
	go util.CopyOutput(ctx, stderrtee, stderrch, diag.Error)

	err = session.Run(cmd)

	stdoutw.Close()
	stderrw.Close()

	<-stdoutch
	<-stderrch

	return stdoutbuf.String(), stderrbuf.String(), id, err
}
