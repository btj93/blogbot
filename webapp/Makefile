.PHONY: install dev build lint deploy

install:
	bun install --frozen-lockfile

dev:
	bun run start

build:
	bun run build

lint:
	bun run lint-staged

deploy:
	./scripts/deploy.sh
