FROM golang:1.17-alpine as build

RUN apk upgrade --no-cache --force
RUN apk add --update build-base make git

WORKDIR /go/src/github.com/webdevops/azure-scheduledevents-manager

# Compile
COPY ./ /go/src/github.com/webdevops/azure-scheduledevents-manager
RUN make dependencies
RUN make test
RUN make build
RUN ./azure-scheduledevents-manager --help

FROM golang:1.17 as kubectl
ARG TARGETOS
ARG TARGETARCH
# kubectl
WORKDIR /
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/$TARGETOS/$TARGETARCH/kubectl
RUN chmod +x /kubectl
RUN /kubectl version --client=true

#############################################
# FINAL IMAGE
#############################################
FROM ubuntu:20.04
ENV LOG_JSON=1
COPY --from=kubectl /kubectl /
COPY --from=build /go/src/github.com/webdevops/azure-scheduledevents-manager/azure-scheduledevents-manager /
USER 1000
ENTRYPOINT ["/azure-scheduledevents-manager"]
