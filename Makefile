BIN := bin/felt

.PHONY: run build test race lint rtp tape clean

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

clean:
	rm -rf bin felt.log demo.gif
