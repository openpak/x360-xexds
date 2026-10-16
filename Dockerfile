# Multi-stage, cross-compiled: the build stage runs on the runner's native arch and Go
# emits the target arch, so linux/arm64 builds need no QEMU.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/x360-xexds ./cmd/x360-xexds \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/x360-xexds-probe ./cmd/x360-xexds-probe

FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app
USER app
COPY --from=build /out/x360-xexds /usr/local/bin/x360-xexds
COPY --from=build /out/x360-xexds-probe /usr/local/bin/x360-xexds-probe
EXPOSE 3074/udp
ENTRYPOINT ["/usr/local/bin/x360-xexds"]
