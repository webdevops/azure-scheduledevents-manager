FROM golang:1.17 as build
WORKDIR /go/src/github.com/webdevops/azure-scheduledevents-manager

# Get deps (cached)
COPY ./go.mod /go/src/github.com/webdevops/azure-scheduledevents-manager
COPY ./go.sum /go/src/github.com/webdevops/azure-scheduledevents-manager
COPY ./Makefile /go/src/github.com/webdevops/azure-scheduledevents-manager
RUN make dependencies

# Compile
COPY ./ /go/src/github.com/webdevops/azure-scheduledevents-manager
RUN make test
RUN make lint
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
