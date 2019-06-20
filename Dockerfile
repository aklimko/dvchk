FROM golang:1.12 as builder

ENV GO111MODULE=on

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build


FROM alpine:3.10

RUN apk add ca-certificates

COPY --from=builder /app/dvchk /app/

ENTRYPOINT ["/app/dvchk"]
