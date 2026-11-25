# Kurze Wege für die tägliche Arbeit. Alles läuft im Go-Container, damit auf dem
# Host nichts installiert werden muss. CPU-Kerne werden automatisch genutzt.
#
#   make test      Tests mit Race-Detector, parallel über alle Kerne
#   make fast      Tests ohne Race (schneller Blick während der Arbeit)
#   make check     was die CI prüft: format, vet, lint, vulncheck, test
#   make fuzz      die Fuzz-Targets kurz durchlaufen lassen
#   make coverage  Coverage-Bericht in coverage.html
#   make image     Container-Image bauen

GO_IMAGE  ?= golang:1.27.4
CPUS      ?= $(shell nproc 2>/dev/null || echo 4)
# Mehr als 16 parallele Tests bringen nichts (Streit um Ports und SQLite-Dateien).
PARALLEL  ?= $(shell n=$$(nproc 2>/dev/null || echo 4); [ $$n -gt 16 ] && echo 16 || echo $$n)
JOBS      ?= $(CPUS)
GOMAXPROCS ?= $(CPUS)
CACHE_VOL := -v $(HOME)/.cache/physiobase-go:/go
# -gcflags/-ldflags bleiben Standard; GOFLAGS ohne -buildvcs, damit der Build-Cache
# (auf NVMe) auch fuer Testbinaries greift.
RUN        = podman run --rm -v "$(CURDIR)":/src $(CACHE_VOL) -w /src \
              -e GOFLAGS=-buildvcs=false -e GOCACHE=/go/build -e GOPATH=/go \
              -e GOMAXPROCS=$(GOMAXPROCS) --cpus=$(CPUS) --memory=8g $(GO_IMAGE)

.PHONY: test fast check fmt vet lint vuln tidy fuzz coverage image clean
.DEFAULT_GOAL := test

test:
	$(RUN) go test -race -shuffle=on -count=1 -p $(JOBS) -parallel $(PARALLEL) -timeout=900s ./...

fast:
	$(RUN) go test -count=1 -p $(JOBS) -parallel $(PARALLEL) -timeout=600s ./...

check: fmt vet lint vuln test

fmt:
	$(RUN) sh -c 'go install mvdan.cc/gofumpt@latest >/dev/null 2>&1 && /go/bin/gofumpt -l -extra .'

vet:
	$(RUN) go vet ./...

lint:
	$(RUN) sh -c 'go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest >/dev/null 2>&1 && /go/bin/golangci-lint run ./...'

vuln:
	$(RUN) sh -c 'go install golang.org/x/vuln/cmd/govulncheck@latest >/dev/null 2>&1 && /go/bin/govulncheck ./...'

tidy:
	$(RUN) go mod tidy

# Kurzes Fuzzing aller Targets, parallel (je 10 s).
fuzz:
	$(RUN) sh -c 'set -e; \
	  ( go test -count=1 -run "^$$" -fuzz "^FuzzParse$$"              -fuzztime=10s ./internal/befund/ ) & \
	  ( go test -count=1 -run "^$$" -fuzz "^FuzzParseQuestionnaire$$" -fuzztime=10s ./internal/befund/ ) & \
	  ( go test -count=1 -run "^$$" -fuzz "^FuzzParseLegacy$$"        -fuzztime=10s ./internal/befund/ ) & \
	  ( go test -count=1 -run "^$$" -fuzz "^FuzzVerify$$"             -fuzztime=10s ./internal/otp/ )    & \
	  ( go test -count=1 -run "^$$" -fuzz "^FuzzNormalize$$"          -fuzztime=10s ./internal/otp/ )    & \
	  wait'

coverage:
	$(RUN) sh -c 'go test -race -count=1 -p $(JOBS) -parallel $(PARALLEL) -covermode=atomic -coverprofile=coverage.out ./... >/dev/null && go tool cover -func=coverage.out | tail -1 && go tool cover -html=coverage.out -o coverage.html'
	@echo "→ coverage.html"

image:
	podman build -t localhost/physiobase:dev .

clean:
	rm -f coverage.out coverage.html
