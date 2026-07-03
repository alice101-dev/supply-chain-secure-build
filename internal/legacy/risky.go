// Package legacy — TEST FILE for the Socket supply-chain scan.
// Every import below is deliberately dangerous: critical CVEs AND abandoned
// upstreams. This PR must never merge.
package legacy

import (
	jwt "github.com/dgrijalva/jwt-go" // CVE-2020-26160: auth bypass; archived
	uuid "github.com/satori/go.uuid"  // CVE-2021-3538: predictable UUIDs; abandoned
	"github.com/tidwall/gjson"        // CVE-2021-42836: ReDoS
)

// Touch each package so `go mod tidy` keeps the risky dependencies.
func Touch(token, json string) string {
	id := uuid.NewV4()
	_, _ = jwt.Parse(token, func(*jwt.Token) (interface{}, error) { return []byte("x"), nil })
	return id.String() + gjson.Get(json, "name").String()
}
