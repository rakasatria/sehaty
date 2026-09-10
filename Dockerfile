# Sehaty — build and run.
#
# Two stages so the shipped image carries a static binary and nothing else: no Go
# toolchain, no package manager, no shell. CGO_ENABLED=0 is what makes that possible.
#
# The SQLite driver is github.com/ncruces/go-sqlite3 — the real SQLite compiled to
# WebAssembly and translated to Go, so it needs no cgo AND can encrypt the database
# file through its Adiantum VFS. The more common cgo-free driver, modernc.org/sqlite,
# cannot encrypt at all, which is why it is not used here.

FROM golang:1.27-alpine AS build
WORKDIR /src
# Copy manifests first so dependency download caches across source edits.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/sehaty ./cmd/sehaty

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/sehaty /sehaty
# SEHATY_DATA is the single mutable root: database and media, and nothing else.
# The KEY is deliberately NOT here — mount it at /run/secrets/sehaty_key. sehaty
# refuses to start if the key file is found inside SEHATY_DATA, because a key in the
# same volume as its ciphertext makes the encryption decoration.
#
# The exercise dataset is a git submodule (MIT, Hasan Emir Yildirim). It may be
# redistributed with attribution, but is kept out of the image to keep it small.
# Bind-mount it read-only at the path below.
ENV SEHATY_DATA=/data \
    SEHATY_EXERCISES=/opt/sehaty/exercises.json \
    SEHATY_FOODS=/opt/sehaty/tkpi-2020.json \
    SEHATY_MCP_HOST=0.0.0.0 \
    SEHATY_MCP_PORT=8765
VOLUME ["/data"]
EXPOSE 8765
# nonroot (uid 65532) — the database and media directory must be writable by it.
USER nonroot:nonroot
ENTRYPOINT ["/sehaty"]
