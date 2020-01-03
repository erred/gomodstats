FROM golang:alpine AS build

ENV CGO_ENABLED=0
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY . .
RUN go build -mod=vendor -o /bin/app


FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /bin/app /

ENTRYPOINT ["/app"]
