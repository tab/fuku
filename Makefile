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
	go test -cover ./...

.PHONY: coverage
coverage:
	@echo "Generating test coverage report..."
	go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out
