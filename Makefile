.PHONY: build

build:
	go get -ldflags "-X github.com/elastic/elastic-package/internal/version.CommitHash=`git describe --always --long --dirty` -X github.com/elastic/elastic-package/internal/version.BuildTime=`date +%s`" \
	    github.com/elastic/elastic-package

clean:
	rm -rf build

format:
	go get -u golang.org/x/tools/cmd/goimports
	goimports -local github.com/elastic/elastic-package/ -w .

lint:
	go get -u golang.org/x/lint/golint
	go list ./... | xargs -n 1 golint -set_exit_status

licenser:
	go get -u github.com/elastic/go-licenser
	go-licenser -license Elastic

gomod:
	go mod tidy

test-stack-command:
	./scripts/test-stack-command.sh

test-check-packages:
	./scripts/test-check-packages.sh

test: test-stack-command test-check-packages

check-git-clean:
	git update-index --really-refresh
	git diff-index --quiet HEAD

check: build format lint licenser gomod test check-git-clean
