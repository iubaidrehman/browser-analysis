.PHONY: build test vet run clean

build:
	go build -o bin/bench ./cmd/bench

test:
	go test ./...

vet:
	go vet ./...

run:
	go run ./cmd/bench

clean:
	rm -rf bin results/raw results/summaries
