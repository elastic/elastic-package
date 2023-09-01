// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package shell

import (
	"net/rpc"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/hashicorp/go-plugin"
)

type Plugin interface {
	Commands() map[string]Command
}

type ShellPlugin struct {
	Impl Plugin
}

func (ShellPlugin) Client(broker *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &ShellPluginRPCClient{
		client: c,
		broker: broker,
	}, nil
}

func (p *ShellPlugin) Server(broker *plugin.MuxBroker) (interface{}, error) {
	return &ShellPluginRPCServer{
		Impl:   p.Impl,
		broker: broker,
	}, nil
}

type ShellPluginRPCClient struct {
	client *rpc.Client
	broker *plugin.MuxBroker
}

type CommandsReply struct {
	M map[string]uint32
}

func (c *ShellPluginRPCClient) Commands() map[string]Command {
	reply := CommandsReply{M: map[string]uint32{}}
	if err := c.client.Call("Plugin.Commands", new(any), &reply); err != nil {
		logger.Error(err)
		return nil
	}

	m := map[string]Command{}

	for k, sid := range reply.M {
		conn, err := c.broker.Dial(sid)
		if err != nil {
			logger.Error(err)
			return nil
		}
		client := rpc.NewClient(conn)
		m[k] = &CommandRPCClient{client: client, broker: c.broker}
	}

	return m
}

type ShellPluginRPCServer struct {
	Impl   Plugin
	broker *plugin.MuxBroker
}

func (s *ShellPluginRPCServer) Commands(_ any, resp *CommandsReply) error {
	commands := s.Impl.Commands()
	if resp.M == nil {
		resp.M = map[string]uint32{}
	}
	for k, c := range commands {
		serverID := s.broker.NextId()
		server := &CommandRPCServer{Impl: c, broker: s.broker}
		resp.M[k] = serverID
		go s.broker.AcceptAndServe(serverID, server)
	}
	return nil
}
