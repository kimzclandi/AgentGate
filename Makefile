.PHONY: build run test race vet check demo bench scan
build:
	mkdir -p bin
	go build -trimpath -o bin/agentgate ./cmd/agentgate
run: build
	AGENTGATE_DEV=1 ./bin/agentgate
test:
	go test -count=1 ./...
race:
	go test -race -count=1 ./...
vet:
	go vet ./...
check: test race vet
demo: build
	python3 scripts/demo.py
bench:
	GOMAXPROCS=4 go test ./internal/gate -run '^$$' -bench . -benchmem -benchtime=2s -count=3
scan:
	go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
