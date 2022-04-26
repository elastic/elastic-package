#!/bin/bash

set -e

curl -s --cacert /usr/share/kibana/config/certs/ca-cert.pem -f https://localhost:5601/login | grep kbn-injected-metadata 2>&1 >/dev/null
curl -s --cacert /usr/share/kibana/config/certs/ca-cert.pem -f -u elastic:changeme "https://elasticsearch:9200/_cat/indices/.security-*?h=health" | grep -v red
