# syntax=docker/dockerfile:1
FROM golang:1.26.3-alpine AS build
WORKDIR /build
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build     CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api     && CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate     && CGO_ENABLED=0 go build -trimpath -o /out/tsc-worker ./cmd/tsc-worker     && CGO_ENABLED=0 go build -trimpath -o /out/supplier-document-storage ./cmd/supplier-document-storage

FROM alpine:3.23 AS runtime
RUN apk add --no-cache python3 py3-pip util-linux ca-certificates libreoffice-writer font-liberation     && addgroup -g 10001 epcerp && adduser -D -u 10001 -G epcerp epcerp
WORKDIR /app
ENV EPCERP_DOCUMENT_PACKAGE_ROOT=/var/lib/epcerp/document-packages
RUN mkdir -p /var/lib/epcerp/document-packages && chmod 0700 /var/lib/epcerp/document-packages && chown 10001:10001 /var/lib/epcerp/document-packages
COPY --from=build /out/api /out/migrate /out/tsc-worker /out/supplier-document-storage ./
COPY tools/pdf-tables/requirements.txt /tmp/pdf-table-requirements.txt
RUN python3 -m venv /opt/pdf-tables && /opt/pdf-tables/bin/pip install --no-cache-dir --only-binary=:all: -r /tmp/pdf-table-requirements.txt && rm /tmp/pdf-table-requirements.txt
COPY tools/pdf-tables/extract.py /app/pdf_tables.py
USER 10001:10001
EXPOSE 8001
CMD ["/app/api"]
