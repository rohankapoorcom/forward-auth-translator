# syntax=docker/dockerfile:1

FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /forward-auth-translator ./cmd/forward-auth-translator

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /forward-auth-translator /forward-auth-translator
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/forward-auth-translator"]
