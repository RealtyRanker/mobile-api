FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

FROM builder AS admin-builder
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /web-admin ./cmd/admin

FROM alpine:3.19 AS web-admin
COPY --from=admin-builder /web-admin /web-admin
EXPOSE 8098
ENTRYPOINT ["/web-admin"]

# ---

FROM builder AS mobile-builder
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /mobile-api ./cmd/mobileapi

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
RUN mkdir -p /var/log/mobile-api

WORKDIR /app
COPY --from=mobile-builder /mobile-api .
COPY config.yaml .

EXPOSE 8090 9097

CMD ["./mobile-api", "-config", "config.yaml"]
