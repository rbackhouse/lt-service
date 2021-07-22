FROM golang:1.16 AS build
WORKDIR /locationtracker

ENV GOPROXY=https://proxy.golang.org
COPY go.mod go.sum /locationtracker/
RUN go mod download

COPY src src
COPY proto proto

RUN CGO_ENABLED=0 GOOS=linux go build -o /go/bin/locationtracker -ldflags="-w -s" -v potpie.org/locationtracker/src/bootstrap

FROM alpine:3.11 AS final
RUN apk --no-cache add ca-certificates
COPY --from=build /go/bin/locationtracker /bin/locationtracker