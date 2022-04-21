#!/bin/bash

set -e

curl -s --cacert /usr/share/kibana/config/certs/ca-cert.pem -f https://localhost:5601/login | grep kbn-injected-metadata 2>&1 >/dev/null
curl -s --cacert /usr/share/kibana/config/certs/ca-cert.pem -f -u elastic:changeme "https://elasticsearch:9200/_cat/indices/.security-*?h=health" | grep -v red

# /!\ HACK to install the CA in agents output. /!\
#
# Alternative would be to pass the CA fingerprint to the configuration file
# and use "xpack.fleet.agents.elasticsearch.ca_sha256" but we need to templatize
# the config file first.
#
# We could also try to make the request from some other place as `elastic-package stack`
# itself, or from an init container, but probably better to templatize the config files.
#
# Another alternative would be to install the outputs from the configuration
# file, with "xpack.fleet.outputs", and pass the ssl config in the config_yaml,
# but it is not possible to set config_yaml in the kibana configuration.
#
if [ ! -f /tmp/.ca_installed ]; then

# Wait for the output to be ready.
curl -s --cacert /usr/share/kibana/config/certs/ca-cert.pem -f -u elastic:changeme https://localhost:5601/api/fleet/outputs/fleet-default-output

# And then overwrite it with the fingerprint.
CA_FINGERPRINT=$(openssl x509 -fingerprint -sha256 -in /usr/share/kibana/config/certs/ca-cert.pem | head -1 | cut -f'2' -d= | sed "s/://g")
curl -s --cacert /usr/share/kibana/config/certs/ca-cert.pem -f -u elastic:changeme -XPUT -H 'Content-Type: application/json' -H 'kbn-xsrf: true' -d@- https://localhost:5601/api/fleet/outputs/fleet-default-output <<EOF
{
  "name": "default",
  "is_default": true,
  "is_default_monitoring": true,
  "type": "elasticsearch",
  "hosts": [
    "https://elasticsearch:9200"
  ],
  "ca_trusted_fingerprint": "$CA_FINGERPRINT",
  "config_yaml": ""
}
EOF

echo $CA_FINGERPRINT > /tmp/.ca_installed
fi
