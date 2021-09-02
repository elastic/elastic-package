#!/bin/bash

set -e

curl -s -f http://127.0.0.1:5601/login | grep kbn-injected-metadata 2>&1 >/dev/null
curl -s -f -u elastic:changeme "http://elasticsearch:9200/_cat/indices/.security-*?h=health" | grep -v red
