default: build

build:
	go build -o bin/generator cmd/generator/main.go cmd/generator/db.go cmd/generator/vm.go