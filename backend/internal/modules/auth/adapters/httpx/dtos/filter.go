package dtos

import (
	"net/url"
	"strconv"
	"unicode/utf8"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

func ParseUserFilter(query url.Values) (domain.UserFilter, error) {
	filter := domain.UserFilter{Page: 1, PageSize: 10}
	var err error
	for _, item := range []struct {
		name   string
		target *int
		max    int
	}{{"page", &filter.Page, 2147483647}, {"page_size", &filter.PageSize, 100}} {
		if values, present := query[item.name]; present {
			value := values[len(values)-1]
			*item.target, err = strconv.Atoi(value)
			if err != nil || *item.target < 1 || *item.target > item.max {
				return domain.UserFilter{}, &Issue{"int_parsing", []string{"query", item.name}, "Invalid pagination value"}
			}
		}
	}
	for _, item := range []struct {
		name   string
		target *string
		max    int
	}{{"search", &filter.Search, 128}, {"role", &filter.Role, 64}} {
		if values, present := query[item.name]; present {
			value := values[len(values)-1]
			if utf8.RuneCountInString(value) < 1 || utf8.RuneCountInString(value) > item.max {
				return domain.UserFilter{}, &Issue{"string_length", []string{"query", item.name}, "Invalid filter length"}
			}
			*item.target = value
		}
	}
	if value, exists := query["status"]; exists {
		active := value[len(value)-1]
		if active != "active" && active != "inactive" {
			return domain.UserFilter{}, &Issue{"string_pattern_mismatch", []string{"query", "status"}, "Invalid status"}
		}
		flag := active == "active"
		filter.Active = &flag
	}
	if value, exists := query["sort"]; exists {
		sort := value[len(value)-1]
		if sort != "username_asc" && sort != "username_desc" {
			return domain.UserFilter{}, &Issue{"string_pattern_mismatch", []string{"query", "sort"}, "Invalid sort"}
		}
		filter.Descending = sort == "username_desc"
	}
	return filter, nil
}
