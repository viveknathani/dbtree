build:
	go build -o ./bin/dbtree ./cmd

test:
	go test -v ./...
