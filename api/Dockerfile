# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/taskboard-api .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/taskboard-api /taskboard-api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/taskboard-api"]
