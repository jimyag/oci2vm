# See the "Complete example" section of the README.
# The build stage compiles the application. The final stage, which oci2vm
# converts, is based on a supported distribution.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -o /hello .

FROM alpine:3.22
COPY --from=build /hello /usr/local/bin/hello
ENV GREETING="Hello from oci2vm"
USER nobody
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/hello"]
