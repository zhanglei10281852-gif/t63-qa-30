.PHONY: test test-race vet build measure frontend-test frontend-build docker-up docker-down

test:
	go test ./... -count=1

test-race:
	go test -race ./... -count=1

vet:
	go vet ./...

build:
	go build ./...

measure:
	go run E:/gomark/.agents/skills/go-base-project-create/scripts/measure_project.go -root . -enforce

frontend-test:
	cd frontend && npm ci && npm test -- --run

frontend-build:
	cd frontend && npm ci && npm run typecheck && npm run build

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down
