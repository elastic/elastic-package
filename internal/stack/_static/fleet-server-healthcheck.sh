#!/bin/bash

set -e -u -o pipefail

NUMBER_SUCCESSES="$1"
WAITING_TIME="$2"

healthcheck() {
    curl -s --cacert /etc/ssl/certs/elastic-package.pem -f https://localhost:8220/api/status | grep -i healthy >/dev/null
}

# Fleet Server can restart after announcing to be healthy, agents connecting during this restart will
# fail to enroll. Expect 3 healthy healthchecks before considering it healthy.
for i in $(seq "$NUMBER_SUCCESSES"); do
	echo "Healthcheck run: $i"
	healthcheck
	sleep "$WAITING_TIME"
done

exit 0
