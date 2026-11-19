.PHONY: market agents web web-dev test tidy docker-up docker-down docker-build

market:
	go run ./cmd/market

agents:
	go run ./cmd/agents

web:
	cd web && npm install && npm run build

web-dev:
	cd web && npm run dev

test:
	go test ./...

tidy:
	go mod tidy

docker-build:
	docker compose build

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down
