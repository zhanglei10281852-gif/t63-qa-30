FROM golang:1.22-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/sanitation-server ./cmd/server

FROM debian:bookworm-slim

RUN useradd --system --uid 10001 sanitation
WORKDIR /app
COPY --from=build /out/sanitation-server /app/sanitation-server
RUN mkdir -p /var/lib/sanitation && chown -R sanitation:sanitation /var/lib/sanitation
USER sanitation
ENV SANITATION_HTTP_ADDRESS=:8653 \
    SANITATION_DATABASE_URL=file:/var/lib/sanitation/sanitation.db \
    SANITATION_ALLOWED_ORIGINS=http://localhost:5173
EXPOSE 8653
ENTRYPOINT ["/app/sanitation-server"]
