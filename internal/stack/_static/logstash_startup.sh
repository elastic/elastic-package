#!/bin/bash

set -euo pipefail

LOGSTASH_HOME="/usr/share/logstash/"

# logstash expects the key in pkcs8 format.
# Hence converting the key.pem to pkcs8 format using openssl.
create_cert() {
  ls_cert_path="$LOGSTASH_HOME/config/certs"
  openssl pkcs8 -inform PEM -in "$ls_cert_path/key.pem" -topk8 -nocrypt -outform PEM -out "$ls_cert_path/logstash.pkcs8.key"
  chmod 777 $ls_cert_path/logstash.pkcs8.key
}

# config copy is intentional that mounted volumes will be busy and cannot be overwritten
overwrite_pipeline_config() {
  ls_pipeline_config_path="$LOGSTASH_HOME/pipeline/"
  cat "$ls_pipeline_config_path/generated_logstash.conf" > "$ls_pipeline_config_path/logstash.conf"
}

# installs the `elastic_integration` plugin if not bundled
install_plugin_if_missing() {
  plugin_name=$1
  if [[ ! $(bin/logstash-plugin list) == *"$plugin_name"* ]]; then
    echo "Missing plugin $plugin_name, installing now"
    bin/logstash-plugin install "$plugin_name"
  fi
}

# runs Logstash
run() {
  bin/logstash -f "$LOGSTASH_HOME/pipeline/logstash.conf" --config.reload.automatic
}

create_cert
overwrite_pipeline_config
install_plugin_if_missing "logstash-filter-elastic_integration"
run
