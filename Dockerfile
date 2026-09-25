FROM node:22-alpine AS frontend

WORKDIR /frontend
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts
COPY tailwind.config.js ./
COPY templates ./templates
COPY static/js ./static/js
COPY static/css/tailwind.input.css ./static/css/tailwind.input.css
RUN npm run build:css

FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend /frontend/static/css/tailwind.css ./static/css/tailwind.css
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/leiheV2 .

FROM alpine:3.22

RUN addgroup -S app && adduser -S -G app app \
    && apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=build /out/leiheV2 ./leiheV2
COPY templates ./templates
COPY static ./static
COPY --from=frontend /frontend/node_modules/alpinejs/dist/cdn.min.js ./static/js/alpine.min.js
COPY --from=frontend /frontend/static/css/tailwind.css ./static/css/tailwind.css

USER app
EXPOSE 8080
ENTRYPOINT ["/app/leiheV2"]
