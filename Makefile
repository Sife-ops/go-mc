default: build

build:
	go build -o bin/generator \
	cmd/generator/main.go \
	cmd/generator/db.go \
	cmd/generator/container.go \
	cmd/generator/seed.go