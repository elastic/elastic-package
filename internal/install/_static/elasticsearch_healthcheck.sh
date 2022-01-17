#!/bin/bash

set -e

PASSWORD_SET_FLAGFILE="/usr/share/elasticsearch/.elasticsearch_password_set"

if [[ ! -f "${PASSWORD_SET_FLAGFILE}" ]]; then
  # Set password for Kibana
  curl -X POST -s -f -u elastic:changeme http://localhost:9200/_security/user/kibana_system/_password \
    --data '{ "password": "kibanapassword" }' -H 'Content-Type: application/json' && \
  touch "${PASSWORD_SET_FLAGFILE}"
fi

curl -s -f -u kibana_system:kibanapassword http://127.0.0.1:9200/