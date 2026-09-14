# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build

WORKDIR /src

# Copy dependency metadata first so Docker can cache module downloads
# independently from application source changes.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/taskboard-api .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/taskboard-api /taskboard-api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/taskboard-api"]
