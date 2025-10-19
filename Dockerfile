FROM golang:1.23-alpine

WORKDIR /app

RUN apk add --no-cache git curl

RUN go install github.com/rakyll/hey@latest

COPY go.mod go.sum* ./

RUN if [ -f go.sum ]; then go mod download; fi

CMD ["tail", "-f", "/dev/null"]
