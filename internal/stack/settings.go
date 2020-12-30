// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import "os"

// Settings encapsulate the various settings required to connect to the Elastic Stack.
type Settings struct {
	Elasticsearch struct {
		Host     string
		Username string
		Password string
	}
	Kibana struct {
		Host string
	}
}

// CurrentSettings loads Elastic stack connection settings from
// environment variables and returns them.
func CurrentSettings() Settings {
	s := Settings{}

	s.Elasticsearch.Host = os.Getenv(ElasticsearchHostEnv)
	if s.Elasticsearch.Host == "" {
		s.Elasticsearch.Host = "http://localhost:9200"
	}

	s.Elasticsearch.Username = os.Getenv(ElasticsearchUsernameEnv)
	s.Elasticsearch.Password = os.Getenv(ElasticsearchPasswordEnv)

	s.Kibana.Host = os.Getenv(KibanaHostEnv)
	if s.Kibana.Host == "" {
		s.Kibana.Host = "http://localhost:5601"
	}

	return s
}
