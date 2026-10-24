package main

import (
	"context"
	"flag"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/adapters/outbound"
	"os"
)

func main() {
	input := flag.String("input", "", "private input path")
	format := flag.String("format", "", "format")
	hash := flag.String("sha256", "", "source checksum")
	flag.Parse()
	b, e := os.ReadFile(*input)
	if e != nil {
		os.Exit(1)
	}
	r, e := (outbound.Parser{}).Read(context.Background(), b, *format, *hash)
	if e != nil {
		os.Exit(1)
	}
	raw, e := (outbound.Codec{}).Encode(r)
	if e != nil {
		os.Exit(1)
	}
	if _, e = os.Stdout.Write(raw); e != nil {
		os.Exit(1)
	}
}
