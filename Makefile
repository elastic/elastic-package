CODE_COVERAGE_REPORT_NAME_UNIT = $(PWD)/build/test-coverage/coverage-unit-report

.PHONY: build

build:
	go get -ldflags "-X github.com/elastic/elastic-package/internal/version.CommitHash=`git describe --always --long --dirty` -X github.com/elastic/elastic-package/internal/version.BuildTime=`date +%s`" \
	    github.com/elastic/elastic-package

clean:
	rm -rf build

format:
	go run golang.org/x/tools/cmd/goimports -local github.com/elastic/elastic-package/ -w .

lint:
	go run honnef.co/go/tools/cmd/staticcheck ./...

licenser:
	go run github.com/elastic/go-licenser -license Elastic

gomod:
	go mod tidy

update-readme:
	cd tools/readme; go run main.go

update: update-readme

test-go:
	# -count=1 is included to invalidate the test cache. This way, if you run "make test-go" multiple times
	# you will get fresh test results each time. For instance, changing the source of mocked packages
	# does not invalidate the cache so having the -count=1 to invalidate the test cache is useful.
	go test -v -count 1 -coverprofile=$(CODE_COVERAGE_REPORT_NAME_UNIT).out ./...

test-go-ci:
	mkdir -p $(PWD)/build/test-results
	mkdir -p $(PWD)/build/test-coverage
	$(MAKE) test-go | go run github.com/tebeka/go2xunit > "$(PWD)/build/test-results/TEST-unit.xml"
	go run github.com/boumenot/gocover-cobertura < $(CODE_COVERAGE_REPORT_NAME_UNIT).out > $(CODE_COVERAGE_REPORT_NAME_UNIT).xml

test-stack-command:
	./scripts/test-stack-command.sh

test-check-packages:
	./scripts/test-check-packages.sh

test-profiles-command:
	./scripts/test-profiles-command.sh

test: test-go test-stack-command test-check-packages test-profiles-command

check-git-clean:
	git update-index --really-refresh
	git diff-index --quiet HEAD

check: check-static test check-git-clean

check-static: build format lint licenser gomod update check-git-clean
