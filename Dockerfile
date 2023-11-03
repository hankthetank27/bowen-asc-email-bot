FROM golang:1.21
WORKDIR /go/src/app
COPY . .
RUN go get -d -v ./...
RUN go build -o email-bot
EXPOSE 3000
CMD ["./email-bot"]
