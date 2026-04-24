default: build

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

fmt:
	gofmt -s -w .

test:
	go test -v -count=1 -parallel=4 ./...

testacc:
	TF_ACC=1 go test -v -count=1 -parallel=4 -timeout 120m ./...

generate:
	go generate ./...

.PHONY: default build install lint fmt test testacc generate
