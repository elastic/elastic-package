package stack

import (
	"errors"
	"os"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

func NewElasticsearchClient(customOptions ...elasticsearch.ClientOption) (*elasticsearch.Client, error) {

	options := []elasticsearch.ClientOption{
		elasticsearch.OptionWithAddress(os.Getenv(ElasticsearchHostEnv)),
		elasticsearch.OptionWithPassword(os.Getenv(ElasticsearchPasswordEnv)),
		elasticsearch.OptionWithUsername(os.Getenv(ElasticsearchUsernameEnv)),
		elasticsearch.OptionWithCertificateAuthority(os.Getenv(CACertificateEnv)),
	}
	options = append(options, customOptions...)
	client, err := elasticsearch.NewClient(options...)

	if errors.Is(err, elasticsearch.ErrUndefinedAddress) {
		return nil, UndefinedEnvError(ElasticsearchHostEnv)
	}

	return client, err
}
