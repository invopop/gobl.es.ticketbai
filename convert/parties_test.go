package convert_test

import (
	"testing"

	"github.com/invopop/gobl.es.ticketbai/convert"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func TestKeepsModelo140(t *testing.T) {
	tests := []struct {
		name string
		code cbc.Code
		want bool
	}{
		{name: "company", code: "B12345674", want: false},
		{name: "entity under income attribution", code: "E12345674", want: true},
		{name: "civil partnership", code: "J12345674", want: true},
		{name: "community of owners", code: "H12345674", want: true},
		{name: "foreign entity", code: "N1234567D", want: false},
		{name: "permanent establishment of a non-resident entity", code: "W1234567D", want: false},
		{name: "national", code: "12345678Z", want: true},
		{name: "foreigner", code: "X1234567L", want: true},
		{name: "other individual", code: "K1234567D", want: true},
		{name: "unrecognised", code: "X34789654", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, convert.KeepsModelo140(&tax.Identity{Country: "ES", Code: tt.code}))
		})
	}
	t.Run("no tax ID", func(t *testing.T) {
		assert.False(t, convert.KeepsModelo140(nil))
	})
}
