FROM golang:1.25-alpine AS build

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /invoice-keeper ./cmd/server

FROM alpine:3.20

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata && mkdir -p /app/data

COPY --from=build /invoice-keeper .
COPY openapi.yaml .

ENV DATABASE_URL=sqlite:///./data/invoice_keeper.db
EXPOSE 8000

CMD ["./invoice-keeper"]
