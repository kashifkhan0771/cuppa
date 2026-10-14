.PHONY: build test lint fmt vet check check-data build-data cover check-dashes release-check release-snapshot sync-packages test-packages

# total test coverage must be above this percent
MIN_COVERAGE := 80

# golangci-lint is built from source with this repo's Go version, so it
# always understands the code it checks; CI uses this same target
GOLANGCI_LINT_VERSION := v2.14.0

# GoReleaser v2 must be installed (brew install goreleaser); building it
# with go run is too slow. CI pins its version in the workflows.
GORELEASER ?= goreleaser

# compile every package; binaries go to bin/
build:
	go build -o bin/ ./...

# run all tests with the race detector on
test:
	go test -race ./...

# run the linters configured in .golangci.yml
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

# rewrite Go files into standard format
fmt:
	gofmt -w .

vet:
	go vet ./...

# the main checks CI runs, in the same order
check: vet lint cover check-dashes

# run all tests, then fail if total coverage is not above MIN_COVERAGE
cover:
	go test -race -covermode=atomic -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | awk -v min=$(MIN_COVERAGE) '/^total:/ { \
		sub("%", "", $$3); \
		print "total coverage: " $$3 "%, must be above " min "%"; \
		if ($$3 + 0 <= min) exit 1 \
	}'

# fail if any file has an en dash or em dash; the bytes are written
# as octal so this Makefile doesn't contain the characters itself
check-dashes:
	@if git grep -nIF --untracked -e "$$(printf '\342\200\223')" -e "$$(printf '\342\200\224')"; then \
		echo "found long dashes above: use a plain hyphen, colon or comma"; \
		exit 1; \
	fi

# validate the question files in questions/
check-data:
	go run ./internal/tools/datagen check

# validate, then write dist/questions.json and dist/categories/
build-data:
	go run ./internal/tools/datagen build

# check .goreleaser.yaml for mistakes
release-check:
	$(GORELEASER) check

# build every release file into build/release without publishing
# (Docker images need a running Docker daemon)
release-snapshot:
	$(GORELEASER) release --snapshot --clean

# copy dist/questions.json and LICENSE into the npm and Python packages
sync-packages:
	cd packages/npm && node scripts/sync-data.mjs
	cp dist/questions.json packages/python/src/cuppa/questions.json
	cp LICENSE packages/npm/LICENSE
	cp LICENSE packages/python/LICENSE

# run the npm and Python package tests (needs Node 18+ and Python 3.9+)
test-packages:
	cd packages/npm && npm ci && npm test
	cd packages/python && PYTHONPATH=src python3 -m unittest discover -s tests
