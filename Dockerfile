FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/dataplanelabs/gcplane/cmd.Version=${VERSION}" -o /gcplane .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates git
COPY --from=builder /gcplane /usr/local/bin/gcplane
ENTRYPOINT ["gcplane"]
