// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
)

func TestApplyResourcesWithHostPorts(t *testing.T) {
	cases := []struct {
		name     string
		config   Config
		expected map[string][]string
	}{
		{
			name:   "fixed host ports",
			config: defaultConfig(),
			expected: map[string][]string{
				"elasticsearch":    {"127.0.0.1:9200:9200"},
				"kibana":           {"127.0.0.1:5601:5601"},
				"package-registry": {"127.0.0.1:8080:8080", "127.0.0.1:9000:9000"},
				"fleet-server":     {"127.0.0.1:8220:8220", "127.0.0.1:8200:8200"},
				"logstash":         {"127.0.0.1:5044:5044", "127.0.0.1:9600:9600"},
			},
		},
		{
			name:   "random host ports",
			config: randomHostPortsConfig(),
			expected: map[string][]string{
				"elasticsearch":    {"127.0.0.1:41001:9200"},
				"kibana":           {"127.0.0.1:41002:5601"},
				"package-registry": {"127.0.0.1:41003:8080", "127.0.0.1:41004:9000"},
				"fleet-server":     {"127.0.0.1:41005:8220", "127.0.0.1:41006:8200"},
				"logstash":         {"127.0.0.1:41007:5044", "127.0.0.1:41008:9600"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ELASTIC_PACKAGE_DATA_HOME", t.TempDir())

			// Enable all the services that publish ports.
			p := createTestProfile(t)
			p.RuntimeOverrides(map[string]string{
				configAPMEnabled:      "true",
				configLogstashEnabled: "true",
			})

			appConfig, err := install.Configuration()
			require.NoError(t, err)

			stackVersion := "8.6.1"
			err = applyResources(p, appConfig, stackVersion, stackVersion, tc.config)
			require.NoError(t, err)

			d, err := os.ReadFile(p.Path(ProfileStackPath, ComposeFile))
			require.NoError(t, err)

			var composeFile struct {
				Services map[string]struct {
					Ports []string `yaml:"ports"`
				} `yaml:"services"`
			}
			err = yaml.Unmarshal(d, &composeFile)
			require.NoError(t, err)

			for service, ports := range tc.expected {
				assert.Equal(t, ports, composeFile.Services[service].Ports, service)
			}
		})
	}
}

func TestSetHostPorts(t *testing.T) {
	config := defaultConfig()

	t.Run("fixed host ports by default", func(t *testing.T) {
		p := createTestProfile(t)

		found, err := setHostPorts(Options{Profile: p}, config)
		require.NoError(t, err)
		assert.Equal(t, config, found)
		assert.False(t, usesRandomHostPorts(found))
	})

	t.Run("unsupported value", func(t *testing.T) {
		p := createTestProfile(t)
		p.RuntimeOverrides(map[string]string{configHostPorts: "other"})

		_, err := setHostPorts(Options{Profile: p}, config)
		assert.ErrorContains(t, err, `unsupported value "other" for stack.host_ports`)
	})
}

func TestShellInitWithHostPorts(t *testing.T) {
	t.Run("fixed host ports before the stack is started", func(t *testing.T) {
		p := createTestProfile(t)

		code, err := ShellInit(p, "bash")
		require.NoError(t, err)
		assert.Contains(t, code, "export ELASTIC_PACKAGE_ELASTICSEARCH_HOST=https://127.0.0.1:9200\n")
		assert.Contains(t, code, "export ELASTIC_PACKAGE_KIBANA_HOST=https://127.0.0.1:5601\n")
	})

	t.Run("random host ports before the stack is started", func(t *testing.T) {
		p := createTestProfile(t)
		p.RuntimeOverrides(map[string]string{configHostPorts: hostPortsRandom})

		_, err := ShellInit(p, "bash")
		assert.ErrorContains(t, err, `the stack of profile "test" uses random host ports and has not been started`)
	})

	t.Run("random host ports after the stack is started", func(t *testing.T) {
		p := createTestProfile(t)
		p.RuntimeOverrides(map[string]string{configHostPorts: hostPortsRandom})
		config := randomHostPortsConfig()
		config.ElasticsearchHost = "https://" + publishedAddress(config, hostPortElasticsearch)
		config.KibanaHost = "https://" + publishedAddress(config, hostPortKibana)
		require.NoError(t, storeConfig(p, config))

		code, err := ShellInit(p, "bash")
		require.NoError(t, err)
		assert.Contains(t, code, "export ELASTIC_PACKAGE_ELASTICSEARCH_HOST=https://127.0.0.1:41001\n")
		assert.Contains(t, code, "export ELASTIC_PACKAGE_KIBANA_HOST=https://127.0.0.1:41002\n")
	})
}

func TestChooseHostPorts(t *testing.T) {
	t.Run("new ports", func(t *testing.T) {
		ports, err := chooseHostPorts(nil, nil)
		require.NoError(t, err)

		chosen := make(map[string]bool)
		for _, published := range publishedPorts {
			hostPort := ports[hostPortKey(published.name)]
			assert.NotEmpty(t, hostPort, published.name)
			chosen[hostPort] = true
		}
		assert.Len(t, chosen, len(publishedPorts), "host ports should be different")
	})

	t.Run("previous ports are kept if they are free", func(t *testing.T) {
		previous, err := chooseHostPorts(nil, nil)
		require.NoError(t, err)

		ports, err := chooseHostPorts(previous, nil)
		require.NoError(t, err)
		assert.Equal(t, previous, ports)
	})

	t.Run("previous ports are replaced if they are not free", func(t *testing.T) {
		previous := listenTestHostPorts(t)

		ports, err := chooseHostPorts(previous, nil)
		require.NoError(t, err)
		for key, hostPort := range previous {
			assert.NotEqual(t, hostPort, ports[key], key)
		}
	})

	t.Run("previous ports are kept if their services are running", func(t *testing.T) {
		previous := listenTestHostPorts(t)

		ports, err := chooseHostPorts(previous, map[string]bool{"elasticsearch": true, "kibana": true})
		require.NoError(t, err)
		for _, published := range publishedPorts {
			key := hostPortKey(published.name)
			switch published.service {
			case "elasticsearch", "kibana":
				assert.Equal(t, previous[key], ports[key], key)
			default:
				assert.NotEqual(t, previous[key], ports[key], key)
			}
		}
	})
}

func TestSetURLs(t *testing.T) {
	p := &profile.Profile{ProfileName: "test"}
	services := func() []ServiceStatus {
		return []ServiceStatus{
			{Name: "elastic-agent", Status: "running (healthy)"},
			{Name: "elasticsearch", Status: "running (healthy)"},
			{Name: "fleet-server", Status: "running (starting)"},
			{Name: "kibana", Status: "running (unhealthy)"},
			{Name: "logstash", Status: "running (healthy)"},
			{Name: "package-registry", Status: "exited (1)"},
		}
	}

	t.Run("fixed host ports", func(t *testing.T) {
		found := services()
		setURLs(found, p, defaultConfig())
		assert.Equal(t, services(), found)
	})

	t.Run("random host ports", func(t *testing.T) {
		found := services()
		setURLs(found, p, randomHostPortsConfig())

		expected := []ServiceStatus{
			{Name: "elastic-agent", Status: "running (healthy)"},
			{Name: "elasticsearch", Status: "running (healthy)", URL: "https://127.0.0.1:41001"},
			{Name: "fleet-server", Status: "running (starting)", URL: "https://127.0.0.1:41005"},
			{Name: "kibana", Status: "running (unhealthy)", URL: "https://test.localhost:41002"},
			{Name: "logstash", Status: "running (healthy)", URL: "127.0.0.1:41007"},
			{Name: "package-registry", Status: "exited (1)"},
		}
		assert.Equal(t, expected, found)
	})
}

func TestPrintKibanaBrowserURL(t *testing.T) {
	p := &profile.Profile{ProfileName: "test"}

	t.Run("fixed host ports", func(t *testing.T) {
		var printer testPrinter
		printKibanaBrowserURL(&printer, p, defaultConfig())
		assert.Empty(t, printer.String())
	})

	t.Run("random host ports", func(t *testing.T) {
		var printer testPrinter
		printKibanaBrowserURL(&printer, p, randomHostPortsConfig())
		assert.Equal(t, "Kibana URL for browsers: https://test.localhost:41002\n", printer.String())
	})
}

// randomHostPortsConfig returns the config of a stack that uses random host ports.
func randomHostPortsConfig() Config {
	return Config{
		Parameters: map[string]string{
			paramHostPorts:                              hostPortsRandom,
			hostPortKey(hostPortElasticsearch):          "41001",
			hostPortKey(hostPortKibana):                 "41002",
			hostPortKey(hostPortPackageRegistry):        "41003",
			hostPortKey(hostPortPackageRegistryMetrics): "41004",
			hostPortKey(hostPortFleetServer):            "41005",
			hostPortKey(hostPortAPMServer):              "41006",
			hostPortKey(hostPortLogstash):               "41007",
			hostPortKey(hostPortLogstashAPI):            "41008",
		},
	}
}

// testPrinter is a printer that keeps what is printed.
type testPrinter struct {
	strings.Builder
}

func (p *testPrinter) Print(i ...interface{})                 { fmt.Fprint(p, i...) }
func (p *testPrinter) Println(i ...interface{})               { fmt.Fprintln(p, i...) }
func (p *testPrinter) Printf(format string, i ...interface{}) { fmt.Fprintf(p, format, i...) }

// listenTestHostPorts returns host ports for all the published ports, that are in use
// during the test.
func listenTestHostPorts(t *testing.T) map[string]string {
	t.Helper()

	ports := make(map[string]string)
	for _, published := range publishedPorts {
		ports[hostPortKey(published.name)] = listenTestPort(t)
	}
	return ports
}

// listenTestPort listens on a free port of the host during the test, and returns the port.
func listenTestPort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", net.JoinHostPort(hostAddress, "0"))
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	return port
}
