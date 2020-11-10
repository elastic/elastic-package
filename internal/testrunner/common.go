package testrunner

import "os"

type StackSettings struct {
	Elasticsearch struct {
		Host     string
		Username string
		Password string
	}
	Kibana struct {
		Host string
	}
}

// GetStackSettingsFromEnv loads Elastic stack connnection settings from
// environment variables and returns them.
func GetStackSettingsFromEnv() StackSettings {
	s := StackSettings{}

	s.Elasticsearch.Host = os.Getenv("ELASTIC_PACKAGE_ELASTICSEARCH_HOST")
	if s.Elasticsearch.Host == "" {
		s.Elasticsearch.Host = "http://localhost:9200"
	}

	s.Elasticsearch.Username = os.Getenv("ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME")
	s.Elasticsearch.Password = os.Getenv("ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD")

	s.Kibana.Host = os.Getenv("ELASTIC_PACKAGE_KIBANA_HOST")
	if s.Kibana.Host == "" {
		s.Kibana.Host = "http://localhost:5601"
	}

	return s
}
