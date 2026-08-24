FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
# CGO off + trimmed path + stripped symbols keeps the pushed layer tiny
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/app .

FROM alpine:3.21
WORKDIR /app
COPY --from=build /out/app ./app
ENTRYPOINT ["./app"]
