.PHONY: test vet fmt fmt-check tidy-check ci tidy

test:
	timeout -k 10s 300s go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

fmt-check:
	@files="$$(gofmt -l .)"; \
	if test -n "$$files"; then \
		printf '%s\n' "$$files"; \
		exit 1; \
	fi

tidy-check:
	go mod tidy -diff

ci: fmt-check tidy-check vet test

tidy:
	go mod tidy
