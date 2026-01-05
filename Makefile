.PHONY: dev dev-backend dev-frontend build run clean test test-backend test-frontend

# Development - run both servers concurrently
dev:
	@trap 'kill 0' EXIT; \
	$(MAKE) dev-backend & \
	$(MAKE) dev-frontend & \
	wait

# Development - run backend and frontend separately
dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

# Build production binary with embedded frontend
build: build-frontend embed-frontend build-backend

build-frontend:
	cd frontend && npm run build

embed-frontend:
	rm -rf backend/cmd/server/dist
	cp -r frontend/dist backend/cmd/server/dist

build-backend:
	cd backend && go build -tags prod -o ../bin/goal-tracker ./cmd/server

# Run production binary
run: build
	./bin/goal-tracker

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf frontend/dist/
	rm -rf backend/cmd/server/dist/

# Testing
test: test-backend test-frontend

test-backend:
	cd backend && go test -v ./...

test-frontend:
	cd frontend && npm run check
