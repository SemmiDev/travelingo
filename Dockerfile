#build API
FROM golang:1.16 as build-go
WORKDIR /travelingo
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY *.go ./
COPY travelingo travelingo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /travel .