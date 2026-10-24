package domain

import (
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

var monetary = regexp.MustCompile(`^[0-9]{1,14}(?:\.[0-9]{1,2})?$`)
var quantity = regexp.MustCompile(`^[0-9]{1,14}(?:\.[0-9]{1,4})?$`)

const maximum int64 = 9999999999999999

func Money(s string) (int64, error) {
	if !monetary.MatchString(s) {
		return 0, Invalid("amount", "Budget amounts must be finite, nonnegative and have at most two decimals")
	}
	p := strings.Split(s, ".")
	v := p[0] + "00"
	if len(p) == 2 {
		v = p[0] + p[1] + strings.Repeat("0", 2-len(p[1]))
	}
	n, e := strconv.ParseInt(v, 10, 64)
	if e != nil || n > maximum {
		return 0, Invalid("amount", "Budget amount exceeds the supported limit")
	}
	return n, nil
}
func Format(n int64) string {
	if n < 0 {
		return "-" + Format(-n)
	}
	return strconv.FormatInt(n/100, 10) + "." + fmtTwo(n%100)
}
func fmtTwo(n int64) string {
	s := strconv.FormatInt(n, 10)
	if n < 10 {
		return "0" + s
	}
	return s
}
func Qty(s string) (*big.Rat, error) {
	if !quantity.MatchString(s) {
		return nil, Invalid("quantity", "Purchase quantity must be positive and finite")
	}
	q, ok := new(big.Rat).SetString(s)
	if !ok || q.Sign() <= 0 {
		return nil, Invalid("quantity", "Purchase quantity must be positive and finite")
	}
	return q, nil
}
func LineBudget(qty, rate string) (string, error) {
	q, e := Qty(qty)
	if e != nil {
		return "", e
	}
	r, e := Money(rate)
	if e != nil {
		return "", e
	}
	v := new(big.Rat).Mul(q, new(big.Rat).SetInt64(r))
	n, rem := new(big.Int), new(big.Int)
	n.QuoRem(v.Num(), v.Denom(), rem)
	if new(big.Int).Mul(rem, big.NewInt(2)).Cmp(v.Denom()) >= 0 {
		n.Add(n, big.NewInt(1))
	}
	if !n.IsInt64() || n.Int64() > maximum {
		return "", Invalid("amount", "Item budget exceeds the supported limit")
	}
	return Format(n.Int64()), nil
}
func addBound(a, b int64) (int64, error) {
	if b > maximum-a {
		return 0, Invalid("amount", "Budget totals exceed the supported limit")
	}
	return a + b, nil
}
func sum(values []int64) (int64, error) {
	var total int64
	for _, v := range values {
		var e error
		total, e = addBound(total, v)
		if e != nil {
			return 0, e
		}
	}
	return total, nil
}
