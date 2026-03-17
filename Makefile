.PHONY: build run test docker-build docker-run clean

build:
	go build -o bin/cloudcut-media-server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

docker-build:
	docker build -t cloudcut-media-server .

docker-run:
	docker run --rm -p 8080:8080 --env-file .env cloudcut-media-server

clean:
	rm -rf bin/
