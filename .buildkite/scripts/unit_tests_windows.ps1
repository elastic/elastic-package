$ErrorActionPreference = "Stop" # set -e

. "$PSScriptRoot\common_windows.ps1"

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
go mod download -x

echo "--- Running unit tests"
$ErrorActionPreference = "Continue" # set +e
go run gotest.tools/gotestsum --junitfile "$(PWD)/TEST-unit-windows.xml" -- -count=1 ./...
$EXITCODE=$LASTEXITCODE
$ErrorActionPreference = "Stop"

Exit $EXITCODE
