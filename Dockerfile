FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/leiheV2 .

FROM alpine:3.22

RUN addgroup -S app && adduser -S -G app app \
    && apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=build /out/leiheV2 ./leiheV2
COPY templates ./templates
COPY static ./static

USER app
EXPOSE 8080
ENTRYPOINT ["/app/leiheV2"]
