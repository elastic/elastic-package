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
    $env:ChocolateyInstall = Convert-Path "$((Get-Command choco).Path)\..\.."
    Import-Module "$env:ChocolateyInstall\helpers\chocolateyProfile.psm1"
    refreshenv
    go version
    go env
}

function installGoDependencies {
    $installPackages = @(
        "github.com/elastic/go-licenser"
        "golang.org/x/tools/cmd/goimports"
        "github.com/jstemmer/go-junit-report/v2"
        "gotest.tools/gotestsum"
    )
    foreach ($pkg in $installPackages) {
        go install "$pkg@latest"
    }
}

fixCRLF
withGolang $env:GO_VERSION


echo "--- Downloading Go modules"
go version
go mod download -x

echo "--- Running unit tests"
go version
$ErrorActionPreference = "Continue" # set +e
go run gotest.tools/gotestsum --junitfile "$(PWD)/TEST-unit.xml" -- -count=1 ./...
$EXITCODE=$LASTEXITCODE
$ErrorActionPreference = "Stop"

mkdir -p build/test-results/

cp "$(PWD)/TEST-unit.xml" build/test-results/TEST-unit.xml

Exit $EXITCODE
