.PHONY: ui build run tidy

ui:
	cd frontend && npm install && npm run build

tidy:
	go mod tidy

build: ui tidy
	go build -o we1bboard.exe ./cmd/we1bboard

run: build
	./we1bboard.exe run
