# Application Information
GIT_COMMIT=$(shell git rev-parse --verify HEAD)
GIT_TAG ?= "master"
RELEASED_AT=$(shell date +'%y.%m.%d-%H:%M:%S')
IMG=opensourceway/omni-orchestrator:$(GIT_COMMIT)
ARCHITECTURE ?= "amd64"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
test: fmt vet swagger-doc
	go test ./... -coverprofile cover.out

# Build manager binary
manager: fmt vet swagger-doc
	go build -ldflags "-X main.Tag=$(GIT_TAG) -X main.CommitID=$(GIT_COMMIT) -X main.ReleaseAt=$(RELEASED_AT)"  -o bin/manager main.go

# Run up server
run: fmt vet swagger-doc
	go run ./main.go

# Generate swagger docs
swagger-doc:
	swag init

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
