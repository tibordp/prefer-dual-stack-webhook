FROM golang:1.17 as build

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY main.go main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o webhook main.go

FROM gcr.io/distroless/base
COPY --from=build /app/webhook /
EXPOSE 8443

ENTRYPOINT ["/webhook"]
