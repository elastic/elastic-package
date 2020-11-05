package dashboards

import (
	"os"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/stack"
)

type Client struct {
	host string
	username string
	password string
}

func NewClient() (*Client, error) {
	host := os.Getenv(stack.KibanaHostEnv)
	if host == "" {
		return nil, stack.UndefinedEnvError(stack.ElasticsearchHostEnv)
	}

	username := os.Getenv(stack.ElasticsearchUsernameEnv)
	password := os.Getenv(stack.ElasticsearchPasswordEnv)

	return &Client{
		host: host,
		username: username,
		password: password,
	}, nil
}

func (c *Client) Export(dashboardIDs []string) ([]common.MapStr, error) {
	return nil, nil
}