FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o product-app product-service/cmd/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/product-app .

EXPOSE 50051

CMD ["./product-app"]
