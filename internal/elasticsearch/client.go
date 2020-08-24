package elasticsearch

import (
	"fmt"
	"os"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/go-elasticsearch/v7"
)

// Client method creates new instance of the Elasticsearch client.
func Client() (*elasticsearch.Client, error) {
	host := os.Getenv(stack.ElasticsearchHostEnv)
	if host == "" {
		return nil, undefinedEnvError(stack.ElasticsearchHostEnv)
	}

	username := os.Getenv(stack.ElasticsearchUsernameEnv)
	password := os.Getenv(stack.ElasticsearchPasswordEnv)

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{host},
		Username:  username,
		Password:  password,
	})
	if err != nil {
		return nil, errors.Wrap(err, "creating Elasticsearch client failed")
	}
	return client, nil
}

func undefinedEnvError(envName string) error {
	return fmt.Errorf("undefined environment variable: %s", envName)
}
