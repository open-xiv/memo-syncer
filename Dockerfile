# phase 1 - build
ARG GO_VERSION
FROM golang:${GO_VERSION:-1.24}-alpine as builder

WORKDIR /app

RUN apk update && apk add --no-cache git ca-certificates && update-ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /server .

# phase 2 - run
FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /server /server

EXPOSE 8080

CMD ["/server"]