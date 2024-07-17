generate:
	go run github.com/steebchen/prisma-client-go generate

push:
	go run github.com/steebchen/prisma-client-go db push

run:
	air cmd/server/main.go

lint:
	golangci-lint run --fast --timeout 3m --config .golangci.yaml

format:
	gofumpt -w -l .
