CODE_COVERAGE_REPORT_FOLDER = $(PWD)/build/test-coverage
CODE_COVERAGE_REPORT_NAME_UNIT = $(CODE_COVERAGE_REPORT_FOLDER)/coverage-unit-report
VERSION_IMPORT_PATH = github.com/elastic/elastic-package/internal/version
VERSION_COMMIT_HASH = `git describe --always --long --dirty`
VERSION_BUILD_TIME = `date +%s`
VERSION_TAG = `(git describe --exact-match --tags 2>/dev/null || echo '') | tr -d '\n'`
VERSION_LDFLAGS = -X $(VERSION_IMPORT_PATH).CommitHash=$(VERSION_COMMIT_HASH) -X $(VERSION_IMPORT_PATH).BuildTime=$(VERSION_BUILD_TIME) -X $(VERSION_IMPORT_PATH).Tag=$(VERSION_TAG)

.PHONY: build

build:
	go build -ldflags "$(VERSION_LDFLAGS)" -o elastic-package

clean:
	rm -rf build
	rm -f elastic-package

format:
	go run golang.org/x/tools/cmd/goimports -local github.com/elastic/elastic-package/ -w .

install:
	go install -ldflags "$(VERSION_LDFLAGS)" github.com/elastic/elastic-package

lint:
	go run honnef.co/go/tools/cmd/staticcheck ./...

licenser:
	go run github.com/elastic/go-licenser -license Elastic

gomod:
	go mod tidy

update-readme:
	cd tools/readme; go run main.go

update: update-readme

$(CODE_COVERAGE_REPORT_FOLDER):
	mkdir -p $@

test-go: $(CODE_COVERAGE_REPORT_FOLDER)
	# -count=1 is included to invalidate the test cache. This way, if you run "make test-go" multiple times
	# you will get fresh test results each time. For instance, changing the source of mocked packages
	# does not invalidate the cache so having the -count=1 to invalidate the test cache is useful.
	go run gotest.tools/gotestsum --format standard-verbose -- -count 1 -coverprofile=$(CODE_COVERAGE_REPORT_NAME_UNIT).out ./...

test-go-ci: $(CODE_COVERAGE_REPORT_FOLDER)
	mkdir -p $(PWD)/build/test-results
	mkdir -p $(PWD)/build/test-coverage
	go run gotest.tools/gotestsum --junitfile "$(PWD)/build/test-results/TEST-unit.xml" -- -count=1 -coverprofile=$(CODE_COVERAGE_REPORT_NAME_UNIT).out ./...
	go run github.com/boumenot/gocover-cobertura < $(CODE_COVERAGE_REPORT_NAME_UNIT).out > $(CODE_COVERAGE_REPORT_NAME_UNIT).xml

test-stack-command-default:
	./scripts/test-stack-command.sh

# Oldest minor where fleet is GA.
test-stack-command-oldest:
	./scripts/test-stack-command.sh 7.14.2

test-stack-command-7x:
	./scripts/test-stack-command.sh 7.17.3-SNAPSHOT

test-stack-command-8x:
	./scripts/test-stack-command.sh 8.3.0-SNAPSHOT

test-stack-command: test-stack-command-default test-stack-command-7x test-stack-command-800 test-stack-command-8x

test-check-packages: test-check-packages-with-kind test-check-packages-other test-check-packages-parallel test-check-packages-with-custom-agent test-check-packages-benchmarks

test-check-packages-with-kind:
	PACKAGE_TEST_TYPE=with-kind ./scripts/test-check-packages.sh

test-check-packages-other:
	PACKAGE_TEST_TYPE=other ./scripts/test-check-packages.sh

test-check-packages-benchmarks:
	PACKAGE_TEST_TYPE=benchmarks ./scripts/test-check-packages.sh

test-check-packages-parallel:
	PACKAGE_TEST_TYPE=parallel ./scripts/test-check-packages.sh

test-check-packages-with-custom-agent:
	PACKAGE_TEST_TYPE=with-custom-agent ./scripts/test-check-packages.sh

test-build-zip:
	./scripts/test-build-zip.sh

test-profiles-command:
	./scripts/test-profiles-command.sh

test: test-go test-stack-command test-check-packages test-profiles-command test-build-zip

check-git-clean:
	git update-index --really-refresh
	git diff-index --quiet HEAD

check: check-static test check-git-clean

check-static: build format lint licenser gomod update check-git-clean
