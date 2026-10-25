FROM golang:1.25-alpine AS build

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /invoice-keeper ./cmd/server \
 && CGO_ENABLED=0 go build -o /smoketest ./cmd/smoketest

FROM alpine:3.20

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata && mkdir -p /app/data

COPY --from=build /invoice-keeper .
COPY --from=build /smoketest .
COPY openapi.yaml .

ENV DATABASE_URL=sqlite:///./data/invoice_keeper.db
EXPOSE 8000

CMD ["./invoice-keeper"]
