FROM golang:1.26.3-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends postgresql postgresql-contrib && rm -rf /var/lib/apt/lists/*
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0
ENV PATH=/usr/local/go/bin:/go/bin:/usr/lib/postgresql/15/bin:$PATH
WORKDIR /workspace
