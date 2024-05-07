#!/bin/bash

set -e

healthcheck() {
    curl --cacert /etc/ssl/elastic-agent/ca-cert.pem -f https://localhost:8220/api/status | grep -i healthy 2>&1 >/dev/null
}

# Fleet Server can restart after announcing to be healthy, agents connecting during this restart will
# fail to enroll. Expect 3 healthy healthchecks before considering it healthy.
expected=3
for i in $(seq $expected); do
	healthcheck
	sleep 1
done

exit 0
