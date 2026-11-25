# Basis-Images per Digest gepinnt (reproduzierbar, unveraenderlich).
FROM mirror.gcr.io/library/golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# -buildvcs=false und -buildid= machen den Build unabhaengig von .git und
# wiederholbar (gleicher Input -> gleiches Binary).
RUN go vet ./... \
 && CGO_ENABLED=0 GOFLAGS=-buildvcs=false go build -trimpath -ldflags='-s -w -buildid=' \
      -o /physiobase ./cmd/physiobase

# Laufzeit: Alpine + statisches Binary. Templates/CSS und die Zeitzonendaten
# stecken im Binary (time/tzdata), deshalb kein apk und kein Netzwerk im Build.
FROM mirror.gcr.io/library/alpine:3.21@sha256:bc21487101ae47172509130ee1092cc273a831968d64b617776d1f69c81e42ab
COPY --from=build /physiobase /usr/local/bin/physiobase
ENV PHYSIOBASE_DATA_DIR=/data PHYSIOBASE_PORT=8080 TZ=Europe/Berlin
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["physiobase", "healthcheck"]
# exec-Form: App ist PID 1 und bekommt SIGTERM direkt (sauberes Herunterfahren, WAL-Checkpoint)
ENTRYPOINT ["/usr/local/bin/physiobase"]
