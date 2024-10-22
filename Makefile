PRECOMMIT_VERSION="3.7.1"
UNAME := $(shell uname)
.PHONY: hooks install install-dev install-test btcli validator-pull miner-pull miner-decentralised miner-centralised validator validator-up-deps miner-worker-api dojo-cli miner-decentralised-logs miner-centralised-logs validator-logs

hooks:
	@echo "Grabbing pre-commit version ${PRECOMMIT_VERSION} and installing pre-commit hooks"
	if [ ! -f pre-commit.pyz ]; then \
		wget -O pre-commit.pyz https://github.com/pre-commit/pre-commit/releases/download/v${PRECOMMIT_VERSION}/pre-commit-${PRECOMMIT_VERSION}.pyz; \
	fi
	python3 pre-commit.pyz clean
	python3 pre-commit.pyz uninstall --hook-type pre-commit --hook-type pre-push --hook-type commit-msg
	python3 pre-commit.pyz gc
	python3 pre-commit.pyz install --hook-type pre-commit --hook-type pre-push --hook-type commit-msg

generate:
	go run github.com/steebchen/prisma-client-go generate

push:
	go run github.com/steebchen/prisma-client-go db push

run:
	air cmd/server/main.go

lint:
	golangci-lint run --fast --timeout 3m --config .golangci.yml

format:
	gofumpt -w -l .
