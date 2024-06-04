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

function withDockerDesktop($version) {
    Write-Host "-- Install docker desktop $version --"
    choco install -y docker-desktop --version $version
    setupChocolateyPath
    docker version
}

function setupChocolateyPath() {
    $env:ChocolateyInstall = Convert-Path "$((Get-Command choco).Path)\..\.."
    Import-Module "$env:ChocolateyInstall\helpers\chocolateyProfile.psm1"
    refreshenv
}


fixCRLF

withGolang $env:GO_VERSION
withDockerDesktop "4.30.0"

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
