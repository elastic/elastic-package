$ErrorActionPreference = "Stop" # set -e

# Forcing to checkout again all the files with a correct autocrlf.
# Doing this here because we cannot set git clone options before.
function fixCRLF {
    Write-Host "-- Fixing CRLF in git checkout --"
    git config core.autocrlf input
    git rm --quiet --cached -r .
    git reset --quiet --hard
}

function withGolang($version) {
    # Avoid conflicts with previous installations.
    Remove-Item env:GOROOT

    Write-Host "-- Install golang $version --"
    choco install -y golang --version $version
    setupChocolateyPath
    go version
    go env
}

function withDocker($version) {
    Write-Host "-- Install Docker $version --"
    choco install -y Containers Microsoft-Hyper-V --source windowsfeatures
    choco install -y docker-engine --version $version
    choco install -y docker-cli --version $version
    setupChocolateyPath
}

function withDockerCompose($version) {
    Write-Host "-- Install Docker Compose $version --"
    choco install -y docker-compose --version $version
    setupChocolateyPath
}

function setupChocolateyPath() {
    $env:ChocolateyInstall = Convert-Path "$((Get-Command choco).Path)\..\.."
    Import-Module "$env:ChocolateyInstall\helpers\chocolateyProfile.psm1"
    refreshenv
}


fixCRLF

withGolang $env:GO_VERSION
# withDocker $env:DOCKER_VERSION
# withDockerCompose $env:DOCKER_COMPOSE_VERSION.Substring(1)

Write-Host "--- Docker Info"
docker info

echo "--- Downloading Go modules"
go mod download -x

echo "--- Running stack tests"
$ErrorActionPreference = "Continue" # set +e

# TODO: stack status checks that we can call docker-compose, but we should try a stack up.
# stack up doesn't work because we didn't manage to enable the linux engine, and we don't have Windows native images.
go run . stack status

$EXITCODE=$LASTEXITCODE
$ErrorActionPreference = "Stop"

Exit $EXITCODE
