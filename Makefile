PROJECT_NAME		:= azure-scheduledevents-manager
GIT_TAG				:= $(shell git describe --dirty --tags --always)
GIT_COMMIT			:= $(shell git rev-parse --short HEAD)
LDFLAGS				:= -X "main.gitTag=$(GIT_TAG)" -X "main.gitCommit=$(GIT_COMMIT)" -extldflags "-static"

FIRST_GOPATH			:= $(firstword $(subst :, ,$(shell go env GOPATH)))
GOLANGCI_LINT_BIN		:= $(FIRST_GOPATH)/bin/golangci-lint

.PHONY: all
all: build

.PHONY: clean
clean:
	git clean -Xfd .

.PHONY: build
build:
	CGO_ENABLED=0 go build -a -ldflags '$(LDFLAGS)' -o $(PROJECT_NAME) .

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor
	go mod verify

.PHONY: image
image: build
	docker build -t $(PROJECT_NAME):$(GIT_TAG) .

.PHONY: build-push-development
build-push-development:
	docker build --build-arg=TARGETOS=$(TARGETOS) --build-arg=TARGETARCH=$(TARGETARCH) --file=Dockerfile -t webdevops/$(PROJECT_NAME):development .
	docker build --build-arg=TARGETOS=$(TARGETOS) --build-arg=TARGETARCH=$(TARGETARCH) --file=Dockerfile.ubuntu -t webdevops/$(PROJECT_NAME):development-ubuntu .
	docker build --build-arg=TARGETOS=$(TARGETOS) --build-arg=TARGETARCH=$(TARGETARCH) --file=Dockerfile.alpine -t webdevops/$(PROJECT_NAME):development-alpine .
	docker build --build-arg=TARGETOS=$(TARGETOS) --build-arg=TARGETARCH=$(TARGETARCH) --file=Dockerfile.kubernetes -t webdevops/$(PROJECT_NAME):development-kubernetes .
	docker build --build-arg=TARGETOS=$(TARGETOS) --build-arg=TARGETARCH=$(TARGETARCH) --file=Dockerfile.distroless -t webdevops/$(PROJECT_NAME):development-distroless .
	docker push	webdevops/$(PROJECT_NAME):development
	docker push	webdevops/$(PROJECT_NAME):development-ubuntu
	docker push	webdevops/$(PROJECT_NAME):development-alpine
	docker push	webdevops/$(PROJECT_NAME):development-kubernetes
	docker push	webdevops/$(PROJECT_NAME):development-distroless

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint: $(GOLANGCI_LINT_BIN)
	$(GOLANGCI_LINT_BIN) run -E exportloopref,gofmt --timeout=10m

.PHONY: dependencies
dependencies: $(GOLANGCI_LINT_BIN)

$(GOLANGCI_LINT_BIN):
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(FIRST_GOPATH)/bin v1.32.2
