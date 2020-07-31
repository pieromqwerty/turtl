FROM golang
RUN mkdir /app
ADD . /app
WORKDIR /app

RUN go build -o main .
CMD ["/app/main"]

EXPOSE 3000