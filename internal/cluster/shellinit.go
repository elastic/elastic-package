package cluster

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"

	"github.com/elastic/elastic-package/internal/install"
)

const shellInitFormat = `ELASTIC_PACKAGE_ELASTICSEARCH_HOST=%s
ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME=%s
ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD=%s
ELASTIC_PACKAGE_KIBANA_HOST=%s`

type kibanaConfiguration struct {
	ElasticsearchHost     string `yaml:"xpack.ingestManager.fleet.elasticsearch.host"`
	ElasticsearchUsername string `yaml:"elasticsearch.username"`
	ElasticsearchPassword string `yaml:"elasticsearch.password"`
	KibanaHost            string `yaml:"xpack.ingestManager.fleet.kibana.host"`
}

// ShellInit method exposes environment variables that can be used for testing purposes.
func ShellInit() (string, error) {
	clusterDir, err := install.ClusterDir()
	if err != nil {
		return "", errors.Wrap(err, "location cluster directory failed")
	}

	kibanaConfigurationPath := filepath.Join(clusterDir, "kibana.config.yml")
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
