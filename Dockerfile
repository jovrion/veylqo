# Multi-stage image for the veylqo process.
# CGO_ENABLED=0 keeps the binary static (ADR-0003: modernc.org/sqlite and pgx).
# The runtime image is distroless static: no shell and no package manager.
# Probe from outside the container: GET /healthz and GET /readyz on port 8080.
# Do not log destination URLs, query strings, IPs, or tokens.

FROM golang:1.27-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0

RUN go build -trimpath -ldflags='-s -w' -o /out/veylqo ./cmd/veylqo

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build --chown=nonroot:nonroot /out/veylqo /veylqo

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/veylqo"]
