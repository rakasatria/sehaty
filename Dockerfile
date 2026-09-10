# Sehaty — build and run.
#
# Two stages so the shipped image carries a static binary and nothing else: no Go
# toolchain, no package manager, no shell. CGO_ENABLED=0 is what makes that possible,
# and is why modernc.org/sqlite was chosen over the cgo sqlite driver.

FROM golang:1.27-alpine AS build
WORKDIR /src
# Copy manifests first so dependency download caches across source edits.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/sehaty ./cmd/sehaty

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/sehaty /sehaty
# The exercise dataset is a submodule and is NOT redistributed — its licence is
# NOASSERTION. Mount it, or run with the submodule checked out and bind-mounted.
ENV SEHATY_DB=/data/sehaty.db \
    SEHATY_MEDIA_DIR=/data/media \
    SEHATY_EXERCISES=/data/exercises.json \
    SEHATY_MCP_HOST=0.0.0.0 \
    SEHATY_MCP_PORT=8765
VOLUME ["/data"]
EXPOSE 8765
# nonroot (uid 65532) — the database and media directory must be writable by it.
USER nonroot:nonroot
ENTRYPOINT ["/sehaty"]
