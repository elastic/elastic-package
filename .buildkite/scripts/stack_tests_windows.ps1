$ErrorActionPreference = "Stop" # set -e

# Forcing to checkout again all the files with a correct autocrlf.
# Doing this here because we cannot set git clone options before.
function fixCRLF {
    Write-Host "-- Fixing CRLF in git checkout --"
    git config core.autocrlf input
    git rm --quiet --cached -r .
    git reset --quiet --hard
}

function ensureBinPath {
    $workDir = if ($env:WORKSPACE) { $env:WORKSPACE } else { $PWD.Path }
    $binDir = Join-Path $workDir "bin"
    if (-not (Test-Path $binDir)) { New-Item -ItemType Directory -Path $binDir | Out-Null }
    $env:PATH = "$binDir;$env:PATH"
    return $binDir
}

function withGolang($version) {
    Write-Host "--- Install golang (GVM)"
    $binDir = ensureBinPath
    $gvmExe = Join-Path $binDir "gvm-windows-amd64.exe"
    $gvmUrl = "https://github.com/andrewkroh/gvm/releases/download/$env:SETUP_GVM_VERSION/gvm-windows-amd64.exe"

    Write-Host "Installing GVM tool"
    $maxTries = 5
    for ($i = 1; $i -le $maxTries; $i++) {
        try {
            Invoke-WebRequest -Uri $gvmUrl -OutFile $gvmExe -UseBasicParsing
            break
        } catch {
            if ($i -eq $maxTries) { throw }
            Start-Sleep -Seconds 3
        }
    }

    Write-Host "Installing Go version $version"
    & $gvmExe --format=powershell $version | Invoke-Expression
    $env:PATH = "$(go env GOPATH)\bin;$env:PATH"
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
    choco install -y docker-compose --allow-downgrade --version $version
    setupChocolateyPath
}

function setupChocolateyPath() {
    $env:ChocolateyInstall = Convert-Path "$((Get-Command choco).Path)\..\.."
    Import-Module "$env:ChocolateyInstall\helpers\chocolateyProfile.psm1"
    refreshenv
}


fixCRLF

withGolang $env:GO_VERSION
withDocker $env:DOCKER_VERSION
withDockerCompose $env:DOCKER_COMPOSE_VERSION.Substring(1)

Write-Host "--- Docker Info"
docker info

echo "--- Downloading Go modules"
go mod download -x

echo "--- Running stack tests"
$ErrorActionPreference = "Continue" # set +e

# TODO: stack status checks that we can call docker, but we should try a stack up to try also with docker-compose with a full scenario.
# stack up doesn't work because we didn't manage to enable the linux engine, and we don't have Windows native images.
echo "Stack Status"
go run . stack status -v
echo "Stack up"
# running this stack up command adds the required files under ~/.elastic-package to run afterwards "elastic-package stack down" successfully
# that uses docker-compose under the hood
go run . stack up -v -d
echo "Stack Status"
go run . stack status -v
echo "Stack down"
go run . stack down -v

$EXITCODE=$LASTEXITCODE
$ErrorActionPreference = "Stop"

Exit $EXITCODE
