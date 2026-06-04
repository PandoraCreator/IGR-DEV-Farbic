# Chaincode-as-a-Service (Go) — build on peer-4 OPS host
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o chaincode .

FROM alpine:3.19
ARG CC_SERVER_PORT=8003
ENV CHAINCODE_SERVER_ADDRESS=0.0.0.0:${CC_SERVER_PORT}
WORKDIR /root
COPY --from=builder /workspace/chaincode .
EXPOSE ${CC_SERVER_PORT}
CMD ["./chaincode"]
