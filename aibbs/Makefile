BINARY=aibbs

.PHONY: build test vet demo clean fmt

build:
	go build -o $(BINARY) ./cmd/aibbs

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

demo: build
	./$(BINARY) demo

clean:
	rm -f $(BINARY)
	rm -rf demo-state
