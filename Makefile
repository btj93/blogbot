.PHONY: build build-webapp test test-webapp lint lint-fix scrape-local showroom-live-local showroom-nextlive-local migrate-blog migrate-showroom migrate-progress deploy ssh

build:
	go build -o blogbot ./cmd/blogbot/

build-webapp:
	cd webapp && bun install --frozen-lockfile && bun run build

test:
	go test ./... -v

test-webapp:
	cd webapp && CI=true bun run test --passWithNoTests

lint:
	golangci-lint run --config .github/.golangci.yml ./...

lint-fix:
	golangci-lint run --config .github/.golangci.yml --fix ./...

scrape-local:
	go run ./cmd/blogbot/ scrape --config config.local.toml

showroom-live-local:
	go run ./cmd/blogbot/ showroom-live --config config.local.toml

showroom-nextlive-local:
	go run ./cmd/blogbot/ showroom-nextlive --config config.local.toml

migrate-blog:
	go run ./cmd/blogbot/ migrate --config config.local.toml --blog-json blog.json

migrate-showroom:
	go run ./cmd/blogbot/ migrate --config config.local.toml --showroom-json showroom.json

migrate-progress:
	go run ./cmd/blogbot/ migrate --config config.local.toml --progress-txt blogProgress.txt

deploy:
	./scripts/deploy.sh

# Set DEPLOY_HOST and DEPLOY_SSH_KEY in your environment; they are deliberately
# not committed.
ssh:
	ssh -i $(DEPLOY_SSH_KEY) $(DEPLOY_HOST)
