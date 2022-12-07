FROM golang:1.18-alpine AS build_base
WORKDIR /go/cs492
COPY go.mod go.sum ./
RUN go mod download

FROM build_base as service_builder
COPY * ./
RUN go build -o /cs492

EXPOSE 8080

CMD [ "/cs492" ]