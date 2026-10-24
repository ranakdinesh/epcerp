package domain

import (
	"regexp"
	"strings"
)

func ClientCode(id string) string {
	return "CLI-" + strings.ToUpper(strings.ReplaceAll(id, "-", "")[:12])
}

var clientStates = map[string]string{
	"01": "Jammu and Kashmir",
	"02": "Himachal Pradesh",
	"03": "Punjab",
	"04": "Chandigarh",
	"05": "Uttarakhand",
	"06": "Haryana",
	"07": "Delhi",
	"08": "Rajasthan",
	"09": "Uttar Pradesh",
	"10": "Bihar",
	"11": "Sikkim",
	"12": "Arunachal Pradesh",
	"13": "Nagaland",
	"14": "Manipur",
	"15": "Mizoram",
	"16": "Tripura",
	"17": "Meghalaya",
	"18": "Assam",
	"19": "West Bengal",
	"20": "Jharkhand",
	"21": "Odisha",
	"22": "Chhattisgarh",
	"23": "Madhya Pradesh",
	"24": "Gujarat",
	"26": "Dadra and Nagar Haveli and Daman and Diu",
	"27": "Maharashtra",
	"29": "Karnataka",
	"30": "Goa",
	"31": "Lakshadweep",
	"32": "Kerala",
	"33": "Tamil Nadu",
	"34": "Puducherry",
	"35": "Andaman and Nicobar Islands",
	"36": "Telangana",
	"37": "Andhra Pradesh",
	"38": "Ladakh",
}

func ClientValidateState(gstin, state string) error {
	if len(gstin) < 2 {
		return ClientInvalid("Invalid GSTIN")
	}
	expected := clientStates[gstin[:2]]
	if expected != "" && ClientNormalize(expected) != ClientNormalize(state) {
		return ClientInvalid("GSTIN state code " + gstin[:2] + " belongs to " + expected + ", but the selected state is " + state)
	}
	return nil
}

var clientPan = regexp.MustCompile(`^[A-Z]{3}[ABCFGHJLPT][A-Z][0-9]{4}[A-Z]$`)
var clientGst = regexp.MustCompile(`^(0[1-9]|[12][0-9]|3[0-8]|97|99)[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$`)

func ClientValidPAN(s string) bool { return clientPan.MatchString(s) && s[5:9] != "0000" }
func ClientValidGST(s string) bool {
	if !clientGst.MatchString(s) || !ClientValidPAN(s[2:12]) {
		return false
	}
	alphabet := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	sum := 0
	for i, r := range s[:14] {
		n := strings.IndexRune(alphabet, r) * (1 + i%2)
		sum += n/36 + n%36
	}
	return s[14] == alphabet[(36-sum%36)%36]
}
