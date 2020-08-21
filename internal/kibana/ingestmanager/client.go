package ingestmanager

import (
	"path"
)

type Client struct {
	apiBaseUrl string

	username string
	password string
}

func NewClient(baseUrl, username, password string) (*Client, error) {
	return &Client{
		path.Join(baseUrl, "api", "ingest_manager"),
		username,
		password,
	}, nil
}
