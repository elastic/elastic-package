// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"os"
	"strings"

	"github.com/elastic/go-resource"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

const (
	hostPortsFixed  = "fixed"
	hostPortsRandom = "random"

	// paramHostPorts is the config parameter set when the stack uses random host ports.
	paramHostPorts = "host_ports"

	// hostAddress is the address of the host where the ports of the stack are published.
	hostAddress = "127.0.0.1"

	// Names of the host ports of the stack.
	hostPortElasticsearch          = "elasticsearch"
	hostPortKibana                 = "kibana"
	hostPortPackageRegistry        = "package_registry"
	hostPortPackageRegistryMetrics = "package_registry_metrics"
	hostPortFleetServer            = "fleet_server"
	hostPortAPMServer              = "apm_server"
	hostPortLogstash               = "logstash"
	hostPortLogstashAPI            = "logstash_api"
)

// publishedPorts are the ports that the compose file of the stack publishes on the host,
// with the names that identify their host ports, and the services that publish them.
var publishedPorts = []struct{ name, service, port string }{
	{hostPortElasticsearch, "elasticsearch", "9200"},
	{hostPortKibana, "kibana", "5601"},
	{hostPortPackageRegistry, "package-registry", "8080"},
	{hostPortPackageRegistryMetrics, "package-registry", "9000"},
	{hostPortFleetServer, "fleet-server", "8220"},
	{hostPortAPMServer, "fleet-server", "8200"},
	{hostPortLogstash, "logstash", "5044"},
	{hostPortLogstashAPI, "logstash", "9600"},
}

// hostPortKey returns the name of the template fact, and of the config parameter, with the
// host port where a port of the stack is published.
func hostPortKey(name string) string {
	return "host_port_" + name
}

// publishedAddress returns the address of the host where a port of a stack that uses random
// host ports is published.
func publishedAddress(config Config, name string) string {
	return net.JoinHostPort(hostAddress, config.Parameters[hostPortKey(name)])
}

// usesRandomHostPorts reports whether the stack with this config uses random host ports.
func usesRandomHostPorts(config Config) bool {
	return config.Parameters[paramHostPorts] == hostPortsRandom
}

// checkHostPortsChosen returns an error if the profile is set to use random host ports and
// its stack has never been started. The addresses of the stack are not known in that case.
func checkHostPortsChosen(profile *profile.Profile) error {
	if profile.Config(configHostPorts, hostPortsFixed) != hostPortsRandom {
		return nil
	}
	_, err := os.Stat(configPath(profile))
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("the stack of profile %q uses random host ports and has not been started, start it with 'elastic-package stack up' to choose them", profile.ProfileName)
	}
	if err != nil {
		return fmt.Errorf("failed to check stack config: %w", err)
	}
	return nil
}

// setHostPorts applies the host ports setting of the profile to the config of a stack that
// is about to start.
func setHostPorts(options Options, config Config) (Config, error) {
	switch hostPorts := options.Profile.Config(configHostPorts, hostPortsFixed); hostPorts {
	case hostPortsFixed:
		return config, nil
	case hostPortsRandom:
	default:
		return Config{}, fmt.Errorf("unsupported value %q for %s, supported values: %s, %s", hostPorts, configHostPorts, hostPortsFixed, hostPortsRandom)
	}

	previous, err := LoadConfig(options.Profile)
	if err != nil {
		return Config{}, fmt.Errorf("failed to load previous stack config: %w", err)
	}
	services, err := dockerComposeStatus(options)
	if err != nil {
		return Config{}, fmt.Errorf("failed to check status of stack services: %w", err)
	}
	running := make(map[string]bool)
	for _, service := range services {
		running[service.Name] = strings.HasPrefix(service.Status, "running")
	}

	config.Parameters, err = chooseHostPorts(previous.Parameters, running)
	if err != nil {
		return Config{}, err
	}
	config.Parameters[paramHostPorts] = hostPortsRandom
	config.ElasticsearchHost = "https://" + publishedAddress(config, hostPortElasticsearch)
	config.KibanaHost = "https://" + publishedAddress(config, hostPortKibana)
	return config, nil
}

// chooseHostPorts returns the config parameters with the host ports to use for a stack,
// given the parameters of its previous config and the services that are running.
func chooseHostPorts(previous map[string]string, running map[string]bool) (map[string]string, error) {
	ports := make(map[string]string)
	for _, published := range publishedPorts {
		key := hostPortKey(published.name)
		if hostPort := previous[key]; hostPort != "" && running[published.service] {
			// The service is already using this port.
			ports[key] = hostPort
			continue
		}

		// Prefer the previous port, so the address doesn't change when the stack is
		// started again.
		listener, err := listenHostPort(previous[key])
		if err != nil {
			return nil, fmt.Errorf("failed to find a free port on the host for %s: %w", published.service, err)
		}
		// Keep the port in use until all of them are chosen, so they are different.
		defer listener.Close()

		_, ports[key], err = net.SplitHostPort(listener.Addr().String())
		if err != nil {
			return nil, fmt.Errorf("failed to read the port chosen for %s: %w", published.service, err)
		}
	}
	return ports, nil
}

// listenHostPort listens on the preferred port of the host if it is free, or on a random
// free port otherwise.
func listenHostPort(preferred string) (net.Listener, error) {
	if preferred != "" {
		listener, err := net.Listen("tcp", net.JoinHostPort(hostAddress, preferred))
		if err == nil {
			return listener, nil
		}
		logger.Debugf("Host port %s is not available anymore, a new one will be used: %v", preferred, err)
	}
	return net.Listen("tcp", net.JoinHostPort(hostAddress, "0"))
}

// hostPortFacts returns the facts used to publish the ports of the stack in the compose file.
func hostPortFacts(config Config) resource.StaticFacter {
	facts := make(resource.StaticFacter)
	for _, published := range publishedPorts {
		hostPort := published.port
		if usesRandomHostPorts(config) {
			hostPort = config.Parameters[hostPortKey(published.name)]
		}
		facts[hostPortKey(published.name)] = hostPort
	}
	return facts
}

// retryWithNewHostPorts starts the stack again if some of its host ports had to be
// replaced after a failed start. Otherwise it returns the error of the failed start.
func retryWithNewHostPorts(ctx context.Context, options Options, agentVersion string, config Config, startErr error) error {
	retryConfig, err := setHostPorts(options, config)
	if err != nil {
		return errors.Join(startErr, fmt.Errorf("choosing host ports again failed: %w", err))
	}
	if maps.Equal(retryConfig.Parameters, config.Parameters) {
		return startErr
	}

	options.Printer.Println("Some host ports are not free anymore, trying again with new ones.")
	printUserConfig(options.Printer, retryConfig)
	printKibanaBrowserURL(options.Printer, options.Profile, retryConfig)

	err = applyResources(options.Profile, options.AppConfig, options.StackVersion, agentVersion, retryConfig)
	if err != nil {
		return fmt.Errorf("creating stack files failed: %w", err)
	}
	err = storeConfig(options.Profile, retryConfig)
	if err != nil {
		return fmt.Errorf("failed to store config: %w", err)
	}
	return dockerComposeUp(ctx, options)
}

// setURLs sets the URLs of the services that are running, if the stack uses random host
// ports.
func setURLs(services []ServiceStatus, profile *profile.Profile, config Config) {
	if !usesRandomHostPorts(config) {
		return
	}

	urls := map[string]string{
		"elasticsearch":    "https://" + publishedAddress(config, hostPortElasticsearch),
		"kibana":           kibanaBrowserURL(profile, config),
		"package-registry": "https://" + publishedAddress(config, hostPortPackageRegistry),
		"fleet-server":     "https://" + publishedAddress(config, hostPortFleetServer),
		// The input for Elastic Agent doesn't use HTTP.
		"logstash": publishedAddress(config, hostPortLogstash),
	}
	for i, service := range services {
		if strings.HasPrefix(service.Status, "running") {
			services[i].URL = urls[service.Name]
		}
	}
}

// kibanaBrowserURL returns the address of Kibana to use in browsers, for a stack that uses
// random host ports.
func kibanaBrowserURL(profile *profile.Profile, config Config) string {
	// Browsers share cookies between the ports of a host, so each profile gets its own
	// hostname to keep the sessions of different stacks apart. Browsers resolve any name
	// under localhost to the loopback address.
	host := net.JoinHostPort(profile.ProfileName+".localhost", config.Parameters[hostPortKey(hostPortKibana)])
	return "https://" + host
}

// printKibanaBrowserURL prints the address of Kibana to use in browsers, if the stack uses
// random host ports.
func printKibanaBrowserURL(printer Printer, profile *profile.Profile, config Config) {
	if printer == nil || !usesRandomHostPorts(config) {
		return
	}
	printer.Printf("Kibana URL for browsers: %s\n", kibanaBrowserURL(profile, config))
}
