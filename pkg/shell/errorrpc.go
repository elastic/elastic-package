// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package shell

import (
	"net/rpc"

	"github.com/elastic/elastic-package/internal/logger"
)

type ErrorRPCClient struct {
	client *rpc.Client
}

type ErrorReply struct {
	Err string
}

func (c *ErrorRPCClient) Error() string {
	var reply ErrorReply
	if err := c.client.Call("Plugin.Error", new(any), &reply); err != nil {
		logger.Error(err)
		return ""
	}
	return reply.Err
}

type ErrorRPCServer struct {
	Impl error
}

func (s *ErrorRPCServer) Error(_ any, resp *ErrorReply) error {
	if s.Impl == nil {
		return nil
	}
	resp.Err = s.Impl.Error()
	return nil
}
