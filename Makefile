build: 
	@go build -o bin/bot

run: build
	@./bin/bot

test:
	@go test -v ./...