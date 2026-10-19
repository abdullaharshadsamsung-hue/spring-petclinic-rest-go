FROM golang:1.25-alpine

WORKDIR /app

ENV GOTOOLCHAIN=local

COPY . .

# go mod tidy completes go.sum for modernc.org/sqlite and jackc/pgx/v5.
RUN go mod tidy && go mod download && go build -o /app/bin/petclinic ./cmd/petclinic

EXPOSE 9966

CMD ["/app/bin/petclinic"]
