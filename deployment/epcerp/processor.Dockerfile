# syntax=docker/dockerfile:1
# A locally verified toolchain cache may replace this stage during offline builds.
ARG PROCESSOR_TOOLCHAIN_IMAGE=toolchain
FROM node:22-bookworm-slim AS node-runtime
FROM debian:trixie-slim AS toolchain
RUN apt-get update && apt-get install -y --no-install-recommends python3 python3-venv ca-certificates poppler-utils tesseract-ocr tesseract-ocr-eng tesseract-ocr-hin tesseract-ocr-osd imagemagick libreoffice-writer-nogui libreoffice-calc-nogui util-linux procps fonts-dejavu-core fonts-noto-core && rm -rf /var/lib/apt/lists/* && groupadd -g 10001 epcerp && useradd -u 10001 -g epcerp -M epcerp
COPY tools/document-processor/requirements.txt /tmp/pdf-table-requirements.txt
RUN python3 -m venv /opt/pdf-tables && /opt/pdf-tables/bin/pip install --no-cache-dir --only-binary=:all: -r /tmp/pdf-table-requirements.txt && rm /tmp/pdf-table-requirements.txt
FROM ${PROCESSOR_TOOLCHAIN_IMAGE} AS runtime
USER root
WORKDIR /app
RUN rm -f /app/api /app/migrate /app/document-worker /app/document-reader /app/document-package-worker /app/tsc-worker /app/supplier-document-storage
COPY --from=node-runtime /usr/local/bin/node /usr/local/bin/node
COPY --chown=10001:10001 tools/document-processor/ /app/
USER 10001:10001
EXPOSE 8082
CMD ["node","/app/server.mjs"]
