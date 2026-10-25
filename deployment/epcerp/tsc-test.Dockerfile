# syntax=docker/dockerfile:1
FROM python:3.13-slim-bookworm@sha256:e32be0e55814a08977501ca863271d9bd02443710db8f2afaf8372e2c4508530
RUN groupadd -g 10001 epcerp-test \
    && useradd -u 10001 -g epcerp-test -M epcerp-test \
    && mkdir /results && chown 10001:10001 /results
WORKDIR /fixtures
COPY testdata/tsc_company/company.json testdata/tsc_company/cases.json testdata/tsc_company/evaluate.py testdata/tsc_company/README.md ./
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
USER 10001:10001
ENTRYPOINT ["python", "/fixtures/evaluate.py"]
