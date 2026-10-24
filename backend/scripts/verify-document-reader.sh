#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
reader_image=${EPCERP_READER_IMAGE:-epcerp-document-worker:local}
reader_arch=$(docker image inspect "$reader_image" --format '{{.Architecture}}')
reader_test_dir=$(mktemp -d)
trap 'rm -rf "$reader_test_dir"' EXIT HUP INT TERM
CGO_ENABLED=0 GOOS=linux GOARCH="$reader_arch" go test -c -o "$reader_test_dir/reader-tests" ./internal/modules/document_reading/adapters/outbound
chmod 755 "$reader_test_dir" "$reader_test_dir/reader-tests"
docker run --rm --network none --read-only --memory 2g --cpus 1 --pids-limit 128 --cap-drop ALL --security-opt no-new-privileges:true --tmpfs /tmp:rw,size=512m,uid=10001,gid=10001,mode=0700 -e EPCERP_READER_TOOLS_TEST=1 -v "$reader_test_dir/reader-tests:/reader-tests:ro" -v "$(pwd)/internal/modules/document_reading/adapters/outbound/testdata:/testdata:ro" -w / "$reader_image" /reader-tests -test.v -test.run '^TestOfflineProductionReaderFormats$' -test.timeout 6m
