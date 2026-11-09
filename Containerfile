FROM docker.io/library/golang:1.26-alpine AS build
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go vet ./... \
 && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /physiobase ./cmd/physiobase

# Laufzeit: Alpine + statisches Binary. Templates/CSS sind eingebettet.
FROM docker.io/library/alpine:3.21
RUN apk add --no-cache tzdata
COPY --from=build /physiobase /usr/local/bin/physiobase
ENV PHYSIOBASE_DATA_DIR=/data PHYSIOBASE_PORT=8080 TZ=Europe/Berlin
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["physiobase", "healthcheck"]
# exec-Form: App ist PID 1 und bekommt SIGTERM direkt (sauberes Herunterfahren, WAL-Checkpoint)
ENTRYPOINT ["/usr/local/bin/physiobase"]
