FROM golang:1.26.0-alpine3.23

WORKDIR /player-tui

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN go build -o player-tui main.go

# Default socket path; override with MPV_SOCKET when using docker-compose.
ENV MPV_SOCKET=/tmp/mpvsocket

CMD ["./player-tui"]