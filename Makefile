.PHONY: build run test docker-build docker-up docker-down clean

build:
	cd ui && npm run build
	go build -o docs-organiser main.go

run: build stop
	./docs-organiser --src ./tmp/source --dst ./tmp/dest

test:
	go test ./internal/... -v

docker-build:
	docker build -t docs-organiser .

docker-up:
	docker-compose up --build

docker-down:
	docker-compose down

stop:
	@echo "[*] Cleaning up existing processes on ports 8081, 8090..."
	@lsof -ti:8081,8090 | xargs kill -9 2>/dev/null || true

clean:
	rm -f docs-organiser
	rm -rf ui/dist

start-local: build stop
	./docs-organiser

dev-ui:
	cd ui && npm run dev

dev-server: stop
	go run main.go

dev: stop
	@echo "[*] Starting unified dev mode (UI on :5173, Server on :8090)..."
	@make -j 2 dev-ui dev-server
