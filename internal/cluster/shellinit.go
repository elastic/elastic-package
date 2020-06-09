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
	ElasticsearchHosts    []string `yaml:"elasticsearch.hosts"`
	ElasticsearchUsername string   `yaml:"elasticsearch.username"`
	ElasticsearchPassword string   `yaml:"elasticsearch.password"`
	KibanaHost            string   `yaml:"xpack.ingestManager.fleet.kibana.host"`
}

func ShellInit() (string, error) {
	clusterPath, err := install.ClusterDir()
	if err != nil {
		return "", errors.Wrap(err, "location cluster configuration failed")
	}

	kibanaConfigurationPath := filepath.Join(clusterPath, "kibana.config.yml")
	body, err := ioutil.ReadFile(kibanaConfigurationPath)
	if err != nil {
		return "", errors.Wrap(err, "reading Kibana configuration file failed")
	}

	var kibanaCfg kibanaConfiguration
	err = yaml.Unmarshal(body, &kibanaCfg)
	if err != nil {
		return "", errors.Wrap(err, "unmarshalling Kibana configuration failed")
	}

	if len(kibanaCfg.ElasticsearchHosts) == 0 {
		return "", errors.New("expected at least one Elasticsearch defined")
	}
	return fmt.Sprintf(shellInitFormat,
		kibanaCfg.ElasticsearchHosts[0],
		kibanaCfg.ElasticsearchUsername,
		kibanaCfg.ElasticsearchPassword,
		kibanaCfg.KibanaHost), nil
}
