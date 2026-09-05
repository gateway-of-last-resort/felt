BIN := bin/felt

.PHONY: run build test short race lint rtp tape snapshot release-check clean

run:
	go run ./cmd/felt

build:
	go build -o $(BIN) ./cmd/felt

test:
	go test ./... -race

# Fast pass: skips the shuffle-distribution and spin-simulation tests.
short:
	go test ./... -short

lint:
	golangci-lint run

# Print the exact figures the tests only assert bands on.
rtp:
	go test ./internal/engine/... -run 'RTP|Return|Edge' -v

tape: build
	vhs demo.tape

# Build every platform's archive without publishing anything.
snapshot:
	goreleaser release --snapshot --clean

# Check the release config without building.
release-check:
	goreleaser check

clean:
	rm -rf bin dist felt.log demo.gif
