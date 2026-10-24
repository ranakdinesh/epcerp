package httpx

import (
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	"net/http"
)

func clientParse(w http.ResponseWriter, r *http.Request) (domain.ClientInput, string, error) {
	return masterDTO.ParseClient(http.MaxBytesReader(w, r.Body, 128<<10))
}
func referenceParse(w http.ResponseWriter, r *http.Request) (domain.ReferenceInput, string, error) {
	return masterDTO.ParseReference(http.MaxBytesReader(w, r.Body, 4<<10))
}
func manufacturerParse(w http.ResponseWriter, r *http.Request, kind string) (masterDTO.ManufacturerInput, bool) {
	in, field, err := masterDTO.ParseManufacturer(http.MaxBytesReader(w, r.Body, 1<<20), kind)
	if err == nil {
		return in, true
	}
	var large *http.MaxBytesError
	if errors.As(err, &large) {
		writeJSON(w, 413, map[string]string{"detail": "Request body too large"})
	} else {
		manufacturerInvalid(w, field)
	}
	return in, false
}
