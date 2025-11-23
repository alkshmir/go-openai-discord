# This dockerfile is optimized for goreleaser and
# you cannot build by bare docker build command.
# Run `go tool goreleaser release --snapshot --clean` to build this image.
FROM gcr.io/distroless/base-debian12:latest
ARG TARGETPLATFORM

USER nonroot
COPY $TARGETPLATFORM/go-openai-discord /usr/bin/go-openai-discord
ENTRYPOINT ["/usr/bin/go-openai-discord"]
