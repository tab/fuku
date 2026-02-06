.PHONY: build
build:
	@echo "Building fuku..."
	go build -o cmd/fuku ./cmd

.PHONY: lint
lint:
	@echo "Running golangci-lint..."
	golangci-lint run

.PHONY: lint\:fix
lint\:fix:
	@echo "Running golangci-lint --fix ..."
	golangci-lint run --fix

.PHONY: vet
vet:
	@echo "Running go vet..."
	go vet ./...

.PHONY: test
test:
	@echo "Running tests..."
	go test -cover $$(go list ./... | grep -v /e2e)

.PHONY: test\:e2e
test\:e2e:
	@echo "Running e2e tests..."
	FUKU_BIN=$(PWD)/cmd/fuku go test -v -timeout 5m ./e2e/...

.PHONY: test\:race
test\:race:
	@echo "Running tests with race detector..."
	go test -race -cover -coverprofile=coverage.out -covermode=atomic $$(go list ./... | grep -v /e2e)

.PHONY: coverage
coverage:
	@echo "Generating test coverage report..."
	go test $$(go list ./... | grep -v /e2e) -coverprofile=coverage.out && go tool cover -html=coverage.out
