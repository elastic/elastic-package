#!/bin/bash

set -e -o pipefail

NUMBER_SUCCESSES=$1
WAITING_TIME=$2

healthcheck() {
    curl -s --cacert /etc/ssl/elastic-agent/ca-cert.pem -f https://localhost:8220/api/status | grep -i healthy 2>&1 >/dev/null
}

# Fleet Server can restart after announcing to be healthy, agents connecting during this restart will
# fail to enroll. Expect 3 healthy healthchecks before considering it healthy.
for i in $(seq "$NUMBER_SUCCESSES"); do
	echo "Iter $i"
	healthcheck
	sleep "$WAITING_TIME"
done

exit 0
