FROM golang:1.25-alpine

WORKDIR /app

COPY . .

RUN go mod download && go build -o /app/bin/server ./cmd/server

EXPOSE 8080

CMD ["sh", "-c", "/app/bin/server"]
