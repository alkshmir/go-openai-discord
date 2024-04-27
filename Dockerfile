# Build stage
FROM golang:1.22 as builder

WORKDIR /build

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build -o main .

# Deploy container
FROM gcr.io/distroless/base-debian12:latest

WORKDIR /

COPY --from=builder /build/main /main
USER nonroot
CMD [ "/main" ]
