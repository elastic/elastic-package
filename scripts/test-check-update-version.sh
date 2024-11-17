#!/bin/bash

set -euxo pipefail

# Last time data modification (seconds since Epoch)
timeLatestDataModification() {
    local filePath="$1"
    stat --format="%Y" ${filePath}
}

latestVersionFilePath="${HOME}/.elastic-package/latestVersion"
rm -rf "${latestVersionFilePath}"

# First usage needs to write the cache file
elastic-package version 

if [ ! -f "${latestVersionFilePath}" ]; then
    echo "Error: Cache file with latest release info not written"
    exit 1
fi

LATEST_MODIFICATION_SINCE_EPOCH=$(timeLatestDataModification "${latestVersionFilePath}")

# Second elastic-package usage should not update the file
elastic-package version

if [ "${LATEST_MODIFICATION_SINCE_EPOCH}" != "$(timeLatestDataModification "${latestVersionFilePath}")" ]; then
    echo "Error: Cache file with latest release info updated - not used cached value"
    exit 1
fi

# If latest data modification is older than the expiration time, it should be updated
# Forced change latest data modification of cache file
cat <<EOF > "${latestVersionFilePath}"
{
    "tag":"v0.85.0",
    "html_url":"https://github.com/elastic/elastic-package/releases/tag/v0.85.0",
    "timestamp":"2023-08-28T17:10:31.735505212+02:00"
}
EOF
LATEST_MODIFICATION_SINCE_EPOCH=$(timeLatestDataModification "${latestVersionFilePath}")

# Precision of stat is in seconds, need to wait at least 1 second
sleep 1

elastic-package version

if [ "${LATEST_MODIFICATION_SINCE_EPOCH}" == "$(timeLatestDataModification "${latestVersionFilePath}")" ]; then
    echo "Error: Cache file with latest release info not updated and timestamp is older than the expiration time"
    exit 1
fi

# If environment variable is defined, cache file should not be written
export ELASTIC_PACKAGE_CHECK_UPDATE_DISABLED=true
rm -rf "${latestVersionFilePath}"

elastic-package version 

if [ -f "${latestVersionFilePath}" ]; then
    echo "Error: Cache file with latest release info written and ELASTIC_PACKAGE_CHECK_UPDATE_DISABLED is defined"
    exit 1
fi