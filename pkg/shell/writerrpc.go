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

type WriterRPCClient struct {
	client *rpc.Client
	broker *plugin.MuxBroker
}

type WriteReply struct {
	N   int
	Err uint32
}

func (c *WriterRPCClient) Write(p []byte) (int, error) {
	var reply WriteReply
	if err := c.client.Call("Plugin.Write", p, &reply); err != nil {
		logger.Error(err)
		return 0, nil
	}

	conn, err := c.broker.Dial(reply.Err)
	if err != nil {
		logger.Error(err)
		return 0, nil
	}
	defer conn.Close()

	client := rpc.NewClient(conn)
	errClient := &ErrorRPCClient{client: client}

	if errStr := errClient.Error(); errStr != "" {
		return reply.N, errors.New(errStr)
	}

	return reply.N, nil
}

type WriterRPCServer struct {
	Impl   io.Writer
	broker *plugin.MuxBroker
}

func (s *WriterRPCServer) Write(p []byte, resp *WriteReply) error {
	n, writeErr := s.Impl.Write(p)

	serverID := s.broker.NextId()
	server := &ErrorRPCServer{Impl: writeErr}
	go s.broker.AcceptAndServe(serverID, server)

	resp.N = n
	resp.Err = serverID
	return nil
}
