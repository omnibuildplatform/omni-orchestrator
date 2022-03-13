
# Image URL to use all building/pushing image targets
IMG=opensourceway/omni-orchestrator:$(shell git rev-parse --verify HEAD)
ARCHITECTURE ?= "amd64"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
test: fmt vet
	go test ./... -coverprofile cover.out

# Build manager binary
manager: fmt vet
	go build -o bin/manager main.go

# Run up server
run: fmt vet
	go run ./main.go

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Build the docker image
docker-build:
	docker build . -t ${IMG} --build-arg GO_ARCHITECTURE=${ARCHITECTURE}

# Push the docker image
docker-push:
	docker push ${IMG}
