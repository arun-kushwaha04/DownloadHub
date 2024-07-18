FROM golang:latest

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go get

RUN go build -o bin .

ENTRYPOINT ["/app/bin"]
