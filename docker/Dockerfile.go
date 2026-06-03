# Build arg: SERVICE_PATH e.g. services/api-gateway
# Preserves monorepo layout so replace ../../shared/go resolves correctly.
FROM golang:1.22-alpine AS builder
ARG SERVICE_PATH
WORKDIR /build
COPY shared/go ./shared/go
COPY ${SERVICE_PATH} ./${SERVICE_PATH}
WORKDIR /build/${SERVICE_PATH}
RUN go mod download && CGO_ENABLED=0 GOOS=linux go build -o /app/service .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/service /app/service
EXPOSE 8080
ENTRYPOINT ["/app/service"]
