build:
	go build -o ./bin/dbtree ./cmd/dbtree/

test:
	go test -v ./...
