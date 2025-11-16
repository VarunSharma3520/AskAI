build:
	@go build -o ./bin/askAI.exe ./cmd/askAI

run: build
	@./bin/askAI.exe
