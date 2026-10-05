package convert_test

import (
	"strings"
	"testing"
	"time"

	"github.com/invopop/gobl.es.ticketbai/convert"
	"github.com/invopop/gobl.es.ticketbai/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFingerprintGeneration(t *testing.T) {
	var conf *convert.Software

	beforeEach := func(t *testing.T) *convert.TicketBAI {
		t.Helper()

		conf = &convert.Software{
			License: "12345",
			NIF:     "12345678A",
			Name:    "My Software",
			Version: "1.0",
		}

		goblInvoice := test.LoadInvoice("sample-invoice.json")

		ts, err := time.Parse(time.RFC3339, "2022-02-01T04:00:00Z")
		require.NoError(t, err)
		role := convert.IssuerRoleThirdParty
		invoice, err := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)
		require.NoError(t, err)
		invoice.Sujetos.Emisor.NIF = test.NIF

		return invoice
	}

	t.Run("should identify the software used to create the tickebai", func(t *testing.T) {
		testCase := beforeEach(t)

		err := testCase.Fingerprint(conf, nil)
		require.NoError(t, err)

		fingerprint := testCase.HuellaTBAI
		assert.NoError(t, err)
		assert.Equal(t, "My Software", fingerprint.Software.Name)
		assert.Equal(t, "1.0", fingerprint.Software.Version)
		assert.Equal(t, "12345678A", fingerprint.Software.NIF)
	})

	t.Run("should not chain invoice if no previous one from the taxpayer", func(t *testing.T) {
		testCase := beforeEach(t)
		err := testCase.Fingerprint(conf, nil)
		require.NoError(t, err)
		fingerprint := testCase.HuellaTBAI
		assert.NoError(t, err)
		assert.Nil(t, fingerprint.EncadenamientoFacturaAnterior)
	})

	t.Run("should chain invoice with the previous one from the taxpayer", func(t *testing.T) {
		testCase := beforeEach(t)

		prev := &convert.ChainData{
			Series:    "A",
			Code:      "1",
			IssueDate: "01-01-2022",
			Signature: strings.Repeat("1234567890", 11),
		}

		err := testCase.Fingerprint(conf, prev)
		require.NoError(t, err)
		fingerprint := testCase.HuellaTBAI
		chaining := fingerprint.EncadenamientoFacturaAnterior
		assert.Equal(t, "1", chaining.NumFacturaAnterior)
		assert.Equal(t, "01-01-2022", chaining.FechaExpedicionFacturaAnterior)
		assert.Equal(t, strings.Repeat("1234567890", 10), chaining.SignatureValueFirmaFacturaAnterior)
	})
}
