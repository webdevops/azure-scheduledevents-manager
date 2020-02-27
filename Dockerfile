FROM golang:1.14 as build

WORKDIR /go/src/github.com/webdevops/azure-scheduledevents-exporter

# Get deps (cached)
COPY ./go.mod /go/src/github.com/webdevops/azure-scheduledevents-exporter
COPY ./go.sum /go/src/github.com/webdevops/azure-scheduledevents-exporter
RUN go mod download

# Compile
COPY ./ /go/src/github.com/webdevops/azure-scheduledevents-exporter
RUN go mod download \
    && CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o /azure-scheduledevents-exporter \
    && chmod +x /azure-scheduledevents-exporter
RUN /azure-scheduledevents-exporter --help

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/static
COPY --from=build /azure-scheduledevents-exporter /
USER 1000
ENTRYPOINT ["/azure-scheduledevents-exporter"]
