FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /nxmc-server ./cmd/nxmc-server

FROM alpine:3.20
COPY --from=build /nxmc-server /nxmc-server
ENTRYPOINT ["/nxmc-server"]
