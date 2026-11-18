// Package lroe holds the rules of Bizkaia's LROE (Libro Registro de
// Operaciones Económicas) shared by the converter and the gateway.
package lroe

import (
	"strings"

	"github.com/invopop/gobl/regimes/es"
	"github.com/invopop/gobl/tax"
)

// KeepsModelo140 reports whether a supplier keeps its LROE under Modelo 140:
// individuals, and entities under the income attribution regime (ERAR) such
// as comunidades de bienes, whose NIF starts with E. Every other supplier,
// including one whose tax ID isn't recognised, keeps Modelo 240.
func KeepsModelo140(tID *tax.Identity) bool {
	switch es.TaxIdentityKey(tID) {
	case es.TaxIdentityNational, es.TaxIdentityForeigner, es.TaxIdentityOther:
		return true
	case es.TaxIdentityOrg:
		return strings.HasPrefix(tID.Code.String(), "E")
	default:
		return false
	}
}
