FROM golang:1.15 as build
ARG TARGETOS=linux
ARG TARGETARCH=amd64
# kubectl
WORKDIR /
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/$TARGETOS/$TARGETARCH/kubectl
RUN chmod +x /kubectl
RUN /kubectl version --client=true --short=true

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

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/base
ENV LOG_JSON=1
COPY --from=build /kubectl /
COPY --from=build /go/src/github.com/webdevops/azure-scheduledevents-manager/azure-scheduledevents-manager /
USER 1000
ENTRYPOINT ["/azure-scheduledevents-manager"]
