FROM golang:1.25-alpine3.23 AS builder

# Private repo
#ENV GOSUMDB="sum.golang.org https://nexus.example.com/repository/go-sum"
#ENV GOPROXY="https://nexus.example.com/repository/go-proxy/"

RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o gitlab-migrate .

FROM alpine:3.23

RUN apk add --no-cache openssh-client ca-certificates tzdata

WORKDIR /app
COPY --from=builder /build/gitlab-migrate .

ENTRYPOINT ["/app/gitlab-migrate"]
CMD ["-config", "config.yaml"]
