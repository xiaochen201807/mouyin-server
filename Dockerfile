FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/mouyin-server ./cmd/mouyin-server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=build /out/mouyin-server /app/mouyin-server

ENV ADDR=:8000
ENV MOUYIN_CACHE_DIR=/var/cache/mouyin

EXPOSE 8000

CMD ["/app/mouyin-server"]
