.PHONY: test test-race lint fmt tidy integration

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy

integration:
	go test -tags=integration ./examples/demo/...
