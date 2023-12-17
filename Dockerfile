#############################################
# Build
#############################################
FROM --platform=$BUILDPLATFORM golang:1.21-alpine as build

RUN apk upgrade --no-cache --force
RUN apk add --update build-base make git curl

WORKDIR /go/src/github.com/webdevops/azure-scheduledevents-manager

# Dependencies
COPY go.mod go.sum .
RUN go mod download

COPY . .
RUN make test
ARG TARGETOS TARGETARCH

# kubectl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x kubectl

# Compile
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} make build

#############################################
# Test
#############################################
FROM gcr.io/distroless/static as test
USER 0:0
WORKDIR /app
COPY --from=build /go/src/github.com/webdevops/azure-scheduledevents-manager/azure-scheduledevents-manager .
COPY --from=build /go/src/github.com/webdevops/azure-scheduledevents-manager/kubectl .
RUN ["./azure-scheduledevents-manager", "--help"]
RUN ["./kubectl", "version", "--client=true"]

#############################################
# Final
#############################################
FROM ubuntu:20.04
ENV LOG_JSON=1
WORKDIR /
COPY --from=test /app /usr/local/bin
USER 1000:1000
ENTRYPOINT ["/usr/local/bin/azure-scheduledevents-manager"]
