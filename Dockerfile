FROM golang:1.26-alpine AS build
WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/client-reminder ./cmd/client-reminder

FROM alpine:3.23
WORKDIR /app
COPY --from=build /bin/client-reminder /usr/local/bin/client-reminder
COPY configs ./configs
COPY state ./state

ENTRYPOINT ["/usr/local/bin/client-reminder"]
