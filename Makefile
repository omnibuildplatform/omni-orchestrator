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
# NOTE(tommylike): do not use go run command for the issue of signal relay issue
run: manager
	./bin/manager

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
	docker build . -t ${IMG} --build-arg GO_ARCHITECTURE=${ARCHITECTURE}  --build-arg GIT_TAG=${GIT_TAG}  --build-arg GIT_COMMIT=${GIT_COMMIT} --build-arg RELEASED_AT=${RELEASED_AT}

# Push the docker image
docker-push:
	docker push ${IMG}

## Prepare cassandra
cassandra:
	./scripts/prepare-database.sh

## deploy yaml
generate:
	cp -rf resources/kubernetes_templates/* ./deploy/resources
	cd ./deploy && kustomize edit set image ${IMG} && kustomize build .
