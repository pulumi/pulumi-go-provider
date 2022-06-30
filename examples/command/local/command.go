package local

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	r "github.com/pulumi/pulumi-go-provider/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-go-provider/examples/command/util"
)

type Command struct {
	// Input
	Interpreter *[]string          `pulumi:"interpreter,optional"`
	Dir         *string            `pulumi:"dir,optional"`
	Environment *map[string]string `pulumi:"environment,optional"`
	Triggers    *[]interface{}     `pulumi:"triggers,optional"`
	Create_     string             `pulumi:"create"`
	Delete_     *string            `pulumi:"delete,optional"`
	Stdin       *string            `pulumi:"stdin,optional"`

	// Output
	Stdout string `pulumi:"stdout" provider:"output"`
	Stderr string `pulumi:"stderr" provider:"output"`
}

// Create executes the create command, sets Stdout and Stderr, and returns a unique ID for
// the command execution
func (c *Command) Create(ctx r.Context, name string, preview bool) (string, error) {
	// TODO: provider interface for generating ids that obey sequence numbers
	if preview {
		ctx.MarkComputed(&c.Stdout)
		ctx.MarkComputed(&c.Stderr)
		return resource.NewUniqueHex("command", 8, 0)
	}
	stdout, stderr, id, err := c.run(ctx, c.Create_)
	c.Stdout = stdout
	c.Stderr = stderr
	return id, err
}

// Delete executes the create command, sets Stdout and Stderr, and returns a unique ID for
// the command execution
func (c *Command) Delete(ctx r.Context, _ r.ID) error {
	if c.Delete_ == nil {
		return nil
	}
	_, _, _, err := c.run(ctx, *c.Delete_)
	return err
}

// run executes the create command, sets Stdout and Stderr, and returns a unique
// ID for the command execution
func (c *Command) run(ctx r.Context, command string) (string, string, string, error) {
	var args []string
	if c.Interpreter != nil && len(*c.Interpreter) > 0 {
		args = append(args, *c.Interpreter...)
	} else {
		if runtime.GOOS == "windows" {
			args = []string{"cmd", "/C"}
		} else {
			args = []string{"/bin/sh", "-c"}
		}
	}
	args = append(args, command)

	stdoutr, stdoutw, err := os.Pipe()
	if err != nil {
		return "", "", "", err
	}
	stderrr, stderrw, err := os.Pipe()
	if err != nil {
		return "", "", "", err
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if cmd == nil {
		return "", "", "", fmt.Errorf("Created null command from ctx (%v)", ctx)
	}
	cmd.Stdout = stdoutw
	cmd.Stderr = stderrw
	if c.Dir != nil {
		cmd.Dir = *c.Dir
	}
	cmd.Env = os.Environ()
	if c.Environment != nil {
		for k, v := range *c.Environment {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	if c.Stdin != nil && len(*c.Stdin) > 0 {
		cmd.Stdin = strings.NewReader(*c.Stdin)
	}

	var stdoutbuf bytes.Buffer
	var stderrbuf bytes.Buffer

	stdouttee := io.TeeReader(stdoutr, &stdoutbuf)
	stderrtee := io.TeeReader(stderrr, &stderrbuf)

	stdoutch := make(chan struct{})
	stderrch := make(chan struct{})
	go util.CopyOutput(ctx, stdouttee, stdoutch, diag.Debug)
	go util.CopyOutput(ctx, stderrtee, stderrch, diag.Error)

	err = cmd.Start()
	if err == nil {
		err = cmd.Wait()
	}

	stdoutw.Close()
	stderrw.Close()

	<-stdoutch
	<-stderrch

	if err != nil {
		return "", "", "", err
	}

	pid := cmd.Process.Pid
	id, err := resource.NewUniqueHex(fmt.Sprintf("%d", pid), 8, 0)
	if err != nil {
		return "", "", "", err
	}

	return strings.TrimSuffix(stdoutbuf.String(), "\n"), strings.TrimSuffix(stderrbuf.String(), "\n"), id, nil
}
