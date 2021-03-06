FROM golang:latest

WORKDIR /go/src/spotify-live-lyricist
COPY . .

RUN go get -d -v ./...
RUN go build -o sll .

CMD ["./sll"]