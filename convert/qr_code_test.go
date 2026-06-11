package convert_test

import (
	"strings"
	"testing"
	"time"

	"github.com/invopop/gobl.ticketbai/convert"
	"github.com/invopop/gobl.ticketbai/test"
	"github.com/invopop/xmldsig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQRCodes(t *testing.T) {
	type TestCase struct {
		invoice *convert.TicketBAI
	}

	ts, err := time.Parse(time.RFC3339, "2022-02-02T04:00:00Z")
	require.NoError(t, err)
	role := convert.IssuerRoleThirdParty

	conf := &convert.Software{
		NIF:     "12345678A",
		Name:    "My Software",
		Version: "1.0",
	}

	cert, err := xmldsig.LoadCertificate(test.Path("test", "certs", "EntitateOrdezkaria_RepresentanteDeEntidad.p12"), "IZDesa2025")
	require.NoError(t, err)

	beforeEach := func(t *testing.T) *TestCase {
		t.Helper()
		goblInvoice := test.LoadInvoice("sample-invoice.json")
		invoice, err := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)
		require.NoError(t, err)

		err = invoice.Fingerprint(conf, nil)
		require.NoError(t, err)

		err = invoice.Sign("TEST", cert, role, convert.ZoneBI, xmldsig.WithCurrentTime(func() time.Time {
			// Make sure same time is always returned so signature values are
			// always the same
			return ts
		}))
		require.NoError(t, err)

		return &TestCase{
			invoice: invoice,
		}
	}

	t.Run("should build TBAI code for an invoice", func(t *testing.T) {
		testCase := beforeEach(t)

		tbai := testCase.invoice
		codes := tbai.QRCodes(convert.ZoneBI)

		assert.Equal(t, 39, len(codes.TBAICode))
		assert.Equal(t, true, strings.HasPrefix(codes.TBAICode, "TBAI-"))
		assert.Contains(t, codes.TBAICode, "-S7836107H-")
		assert.Contains(t, codes.TBAICode, "-020222-")
		assert.Contains(t, codes.TBAICode, "-JzKI54fHylY3s-")
		assert.Contains(t, codes.TBAICode, "-153")
	})

	t.Run("should build QR code for an invoice", func(t *testing.T) {
		testCase := beforeEach(t)

		tbai := testCase.invoice
		codes := tbai.QRCodes(convert.ZoneBI)

		assert.Equal(t, true, strings.HasPrefix(codes.QRCode, "https://batuz.eus/QRTBAI/"))
		assert.Contains(t, codes.QRCode, "?id=TBAI-S7836107H-020222-")
		assert.Contains(t, codes.QRCode, "&s=TEST")
		assert.Contains(t, codes.QRCode, "&nf=001")
		assert.Contains(t, codes.QRCode, "&i=1089.00")
		assert.Contains(t, codes.QRCode, "&cr=010") // changes according to test data
	})

	t.Run("should build QR code for an invoice without series", func(t *testing.T) {
		testCase := beforeEach(t)

		tbai := testCase.invoice
		tbai.Factura.CabeceraFactura.SerieFactura = ""
		codes := tbai.QRCodes(convert.ZoneBI)

		assert.Equal(t, true, strings.HasPrefix(codes.QRCode, "https://batuz.eus/QRTBAI/"))
		assert.Contains(t, codes.QRCode, "?id=TBAI-S7836107H-020222-")
		assert.NotContains(t, codes.QRCode, "&s=TEST")
		assert.Contains(t, codes.QRCode, "&nf=001")
		assert.Contains(t, codes.QRCode, "&i=1089.00")
		assert.Contains(t, codes.QRCode, "&cr=232")
	})
}
