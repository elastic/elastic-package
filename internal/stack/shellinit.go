package stack

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/install"
)

const (
	elasticPackageEnvPrefix = "ELASTIC_PACKAGE_"
)

// Environment variables describing the stack.
var (
	ElasticsearchHostEnv     = elasticPackageEnvPrefix + "ELASTICSEARCH_HOST"
	ElasticsearchUsernameEnv = elasticPackageEnvPrefix + "ELASTICSEARCH_USERNAME"
	ElasticsearchPasswordEnv = elasticPackageEnvPrefix + "ELASTICSEARCH_PASSWORD"
	KibanaHostEnv            = elasticPackageEnvPrefix + "KIBANA_HOST"
)

var shellInitFormat = ElasticsearchHostEnv + "=%s\n" + ElasticsearchUsernameEnv + "=%s\n" +
	ElasticsearchPasswordEnv + "=%s\n" + KibanaHostEnv + "=%s\n"

type kibanaConfiguration struct {
	ElasticsearchHost     string `yaml:"xpack.ingestManager.fleet.elasticsearch.host"`
	ElasticsearchUsername string `yaml:"elasticsearch.username"`
	ElasticsearchPassword string `yaml:"elasticsearch.password"`
	KibanaHost            string `yaml:"xpack.ingestManager.fleet.kibana.host"`
}

// ShellInit method exposes environment variables that can be used for testing purposes.
func ShellInit() (string, error) {
	stackDir, err := install.StackDir()
	if err != nil {
		return "", errors.Wrap(err, "locating stack directory failed")
	}

	kibanaConfigurationPath := filepath.Join(stackDir, "kibana.config.yml")
	body, err := ioutil.ReadFile(kibanaConfigurationPath)
	if err != nil {
		return "", errors.Wrap(err, "reading Kibana configuration file failed")
	}

	var kibanaCfg kibanaConfiguration
	err = yaml.Unmarshal(body, &kibanaCfg)
	if err != nil {
		return "", errors.Wrap(err, "unmarshalling Kibana configuration failed")
	}
	return fmt.Sprintf(shellInitFormat,
		kibanaCfg.ElasticsearchHost,
		kibanaCfg.ElasticsearchUsername,
		kibanaCfg.ElasticsearchPassword,
		kibanaCfg.KibanaHost), nil
}
