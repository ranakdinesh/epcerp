FROM python:3.12-slim-bookworm AS build
ARG TARGETARCH
ENV TARGETARCH=$TARGETARCH PIP_DISABLE_PIP_VERSION_CHECK=1
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /build
COPY reranker/requirements.txt ./
RUN pip install --no-cache-dir --prefix=/install -r requirements.txt
COPY reranker/prepare.py ./
RUN python prepare.py

FROM python:3.12-slim-bookworm
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 HF_HUB_OFFLINE=1 TOKENIZERS_PARALLELISM=false OMP_NUM_THREADS=1
COPY --from=build /install /usr/local
COPY --from=build /model /model
WORKDIR /app
COPY reranker/server.py reranker/smoke.py reranker/guardrails.py ./
USER 10001:10001
CMD ["python","server.py"]
