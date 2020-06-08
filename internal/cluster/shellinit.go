package cluster

import (
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
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
	elasticPackagePath := filepath.Join("/Users/marcin.tojek/.elastic-package", "cluster", "kibana.config.yml")
	body, err := ioutil.ReadFile(elasticPackagePath)
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
