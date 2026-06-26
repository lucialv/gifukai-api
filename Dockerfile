FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE_VERSION=dev
RUN CGO_ENABLED=0 GOFLAGS=-trimpath go build \
    -ldflags="-s -w" -o /out/gifukai-api ./cmd/main.go

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/gifukai-api /gifukai-api
EXPOSE 8001
ENV ENV=production PORT=8001
USER nonroot:nonroot
ENTRYPOINT ["/gifukai-api"]
