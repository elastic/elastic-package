// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package shell

import (
	"errors"
	"io"
	"net/rpc"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/hashicorp/go-plugin"
)

type Command interface {
	// Usage is the one-line usage message.
	// Recommended syntax is as follows:
	//   [ ] identifies an optional argument. Arguments that are not enclosed in brackets are required.
	//   ... indicates that you can specify multiple values for the previous argument.
	//   |   indicates mutually exclusive information. You can use the argument to the left of the separator or the
	//       argument to the right of the separator. You cannot use both arguments in a single use of the command.
	//   { } delimits a set of mutually exclusive arguments when one of the arguments is required. If the arguments are
	//       optional, they are enclosed in brackets ([ ]).
	// Example: add [-F file | -D dir]... [-f format] profile
	Usage() string
	Name() string
	Desc() string
	Exec(wd string, args []string, stdout, stderr io.Writer) error
}

type CommandRPCClient struct {
	client *rpc.Client
	broker *plugin.MuxBroker
}

type ExecArgs struct {
	Wd             string
	Args           []string
	Stdout, Stderr uint32
}

type ExecReply struct {
	Err uint32
}

func (c *CommandRPCClient) Usage() string {
	return c.getStrCall("Plugin.Usage")
}

func (c *CommandRPCClient) Name() string {
	return c.getStrCall("Plugin.Name")
}

func (c *CommandRPCClient) Desc() string {
	return c.getStrCall("Plugin.Desc")
}

func (c *CommandRPCClient) getStrCall(method string) string {
	var reply string
	if err := c.client.Call(method, new(any), &reply); err != nil {
		logger.Error(err)
		return ""
	}
	return reply
}

func (c *CommandRPCClient) Exec(wd string, args []string, stdout, stderr io.Writer) error {
	stdoutServerID := c.broker.NextId()
	stdoutServer := &WriterRPCServer{Impl: stdout, broker: c.broker}
	go c.broker.AcceptAndServe(stdoutServerID, stdoutServer)

	stderrServerID := c.broker.NextId()
	stderrServer := &WriterRPCServer{Impl: stderr, broker: c.broker}
	go c.broker.AcceptAndServe(stderrServerID, stderrServer)

	var reply ExecReply
	if err := c.client.Call("Plugin.Exec", ExecArgs{Wd: wd, Args: args, Stdout: stdoutServerID, Stderr: stderrServerID}, &reply); err != nil {
		logger.Error(err)
		return nil
	}

	conn, err := c.broker.Dial(reply.Err)
	if err != nil {
		logger.Error(err)
		return nil
	}
	defer conn.Close()

	client := rpc.NewClient(conn)
	errClient := &ErrorRPCClient{client: client}

	if errStr := errClient.Error(); errStr != "" {
		return errors.New(errStr)
	}

	return nil
}

type CommandRPCServer struct {
	Impl   Command
	broker *plugin.MuxBroker
}

func (s *CommandRPCServer) Usage(_ any, resp *string) error {
	*resp = s.Impl.Usage()
	return nil
}

func (s *CommandRPCServer) Name(_ any, resp *string) error {
	*resp = s.Impl.Name()
	return nil
}

func (s *CommandRPCServer) Desc(_ any, resp *string) error {
	*resp = s.Impl.Desc()
	return nil
}

func (s *CommandRPCServer) Exec(args ExecArgs, resp *ExecReply) error {
	stdoutConn, err := s.broker.Dial(args.Stdout)
	if err != nil {
		return err
	}
	defer stdoutConn.Close()
	stdoutClient := rpc.NewClient(stdoutConn)

	stderrConn, err := s.broker.Dial(args.Stderr)
	if err != nil {
		return err
	}
	defer stderrConn.Close()
	stderrClient := rpc.NewClient(stderrConn)

	execErr := s.Impl.Exec(args.Wd, args.Args, &WriterRPCClient{client: stdoutClient, broker: s.broker}, &WriterRPCClient{client: stderrClient, broker: s.broker})

	serverID := s.broker.NextId()
	server := &ErrorRPCServer{Impl: execErr}
	go s.broker.AcceptAndServe(serverID, server)

	resp.Err = serverID

	return nil
}
