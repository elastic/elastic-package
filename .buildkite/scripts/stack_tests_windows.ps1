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
    Write-Host "-- Install golang $version --"
    choco install -y golang --version $version
    setupChocolateyPath
    go version
    go env
}

function withDocker($version) {
    Write-Host "-- Install docker CLI $version --"
    choco install -y docker-cli --version $version
    setupChocolateyPath
    docker version
}

function withDockerCompose($version) {
    Write-Host "-- Install docker-compose $version --"
    choco install -y docker-compose --version $version
    setupChocolateyPath
    docker compose version
}

function setupChocolateyPath() {
    $env:ChocolateyInstall = Convert-Path "$((Get-Command choco).Path)\..\.."
    Import-Module "$env:ChocolateyInstall\helpers\chocolateyProfile.psm1"
    refreshenv
}


fixCRLF

withGolang $env:GO_VERSION
withDocker $env:DOCKER_VERSION # Dependency of docker-compose in chocolatey.
withDockerCompose $env:DOCKER_COMPOSE_VERSION.Trim("v")

echo "--- Docker Info"
docker info

echo "--- Downloading Go modules"
go version
go mod download -x

echo "--- Running stack tests"
go version
$ErrorActionPreference = "Continue" # set +e
go run . stack up -v -d
$EXITCODE=$LASTEXITCODE
$ErrorActionPreference = "Stop"

Exit $EXITCODE
