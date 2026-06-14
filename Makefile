.PHONY: build run test coverage lint clean tidy

build:
	go build -o ollama-fit .

run:
	go run .

test:
	go test ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

lint:
	go vet ./...

clean:
	rm -f ollama-fit coverage.out coverage.html

tidy:
	go mod tidy
