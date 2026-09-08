FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bioconnect ./cmd/bioconnect
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata poppler-utils && rm -rf /var/lib/apt/lists/* && useradd -u 10001 -m app && mkdir -p /data/files && chown -R app:app /data
COPY --from=build /out/bioconnect /usr/local/bin/bioconnect
USER app
WORKDIR /data
EXPOSE 8080
ENTRYPOINT ["bioconnect"]
