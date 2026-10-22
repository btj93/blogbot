.PHONY: install dev build lint fmt deploy

install:
	bun install --frozen-lockfile

dev:
	bun run start

build:
	bun run build

lint:
	bun run fmt:check && bun run lint

fmt:
	bun run fmt

deploy:
	./scripts/deploy.sh
