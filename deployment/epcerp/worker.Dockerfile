# syntax=docker/dockerfile:1
FROM golang:1.26.3-alpine AS build
WORKDIR /build
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -trimpath -o /out/document-worker ./cmd/document-worker && CGO_ENABLED=0 go build -trimpath -o /out/document-reader ./cmd/document-reader
FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates poppler-utils tesseract-ocr tesseract-ocr-eng tesseract-ocr-hin tesseract-ocr-osd imagemagick libreoffice-writer-nogui libreoffice-calc-nogui util-linux procps fonts-dejavu-core fonts-noto-core && rm -rf /var/lib/apt/lists/* && groupadd -g 10001 epcerp && useradd -u 10001 -g epcerp -M epcerp
WORKDIR /app
COPY --from=build /out/document-worker /out/document-reader ./
USER 10001:10001
CMD ["/app/document-worker"]
