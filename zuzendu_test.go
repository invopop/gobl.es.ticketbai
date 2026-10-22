package ticketbai_test

import (
	"context"
	"testing"
	"time"

	ticketbai "github.com/invopop/gobl.es.ticketbai"
	"github.com/invopop/gobl.es.ticketbai/convert"
	"github.com/invopop/gobl.es.ticketbai/internal/gateways"
	"github.com/invopop/gobl.es.ticketbai/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/lestrrat-go/helium/xsd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZuzendu(t *testing.T) {
	schema, err := xsd.NewCompiler().CompileFile(
		context.Background(),
		test.Path("test", "schema", "zuzendu-schema.xsd"),
	)
	require.NoError(t, err)

	tc, err := loadTBAIClient()
	require.NoError(t, err)

	// issueOriginal signs the invoice and returns the original document as it
	// would be loaded back from storage.
	issueOriginal := func(t *testing.T) (*convert.TicketBAI, []string) {
		t.Helper()
		env := test.LoadEnvelope("invoice-ss.json")
		td, err := tc.Convert(env)
		require.NoError(t, err)
		require.NoError(t, tc.Fingerprint(td, &convert.ChainData{
			Series:    "AF",
			Code:      "1234567890",
			IssueDate: "01-01-2021",
			Signature: "previous-signature",
		}))
		require.NoError(t, tc.Sign(td, env))
		data, err := td.Bytes()
		require.NoError(t, err)
		orig, err := ticketbai.ParseDocument(data)
		require.NoError(t, err)

		stamps := make([]string, 0, len(env.Head.Stamps))
		for _, s := range env.Head.Stamps {
			stamps = append(stamps, s.Value)
		}
		return orig, stamps
	}

	t.Run("modifies an issued invoice", func(t *testing.T) {
		orig, _ := issueOriginal(t)

		// Re-generating happens later, with corrected details and the
		// stamps from the original issue still in place.
		env := test.LoadEnvelope("invoice-ss.json")
		inv := env.Extract().(*bill.Invoice)
		inv.Customer.Name = "Corrected Name SL"
		later, err := loadTBAIClientAt(time.Date(2022, 3, 15, 10, 0, 0, 0, time.UTC))
		require.NoError(t, err)

		zd, err := later.GenerateZuzendu(env, orig, ticketbai.ZuzenduModify)
		require.NoError(t, err)

		assert.Equal(t, convert.ZuzenduModify, zd.Cabecera.Accion)
		assert.Equal(t, "Corrected Name SL", zd.Sujetos.Destinatarios.IDDestinatario[0].ApellidosNombreRazonSocial)
		assert.Equal(t, orig.Head().FechaExpedicionFactura, zd.Factura.CabeceraFactura.FechaExpedicionFactura)
		assert.Equal(t, orig.Head().HoraExpedicionFactura, zd.Factura.CabeceraFactura.HoraExpedicionFactura)
		assert.Equal(t, orig.HuellaTBAI, zd.HuellaTBAI)
		assert.Len(t, zd.SignatureValueFirmaFactura, 100)
		assert.Equal(t, orig.SignatureValue()[:100], zd.SignatureValueFirmaFactura)

		data, err := zd.BytesIndent()
		require.NoError(t, err)
		require.NoError(t, validateDoc(schema, data))
	})

	t.Run("corrects a rejected invoice", func(t *testing.T) {
		orig, stamps := issueOriginal(t)
		env := test.LoadEnvelope("invoice-ss.json")

		zd, err := tc.GenerateZuzendu(env, orig, ticketbai.ZuzenduCorrect)
		require.NoError(t, err)
		assert.Equal(t, convert.ZuzenduCorrect, zd.Cabecera.Accion)

		data, err := zd.Bytes()
		require.NoError(t, err)
		require.NoError(t, validateDoc(schema, data))

		// The TicketBAI code and QR come from the original signature, so
		// they remain valid for the corrected document.
		assert.Contains(t, stamps[0], orig.SignatureValue()[:13])
	})

	t.Run("rejects a different invoice", func(t *testing.T) {
		orig, _ := issueOriginal(t)
		env := test.LoadEnvelope("invoice-ss.json")
		env.Extract().(*bill.Invoice).Code = cbc.Code("OTHER")

		_, err := tc.GenerateZuzendu(env, orig, ticketbai.ZuzenduModify)
		assert.ErrorIs(t, err, ticketbai.ErrValidation)
		assert.ErrorContains(t, err, "invoice code does not match")
	})

	t.Run("rejects an unsigned original", func(t *testing.T) {
		env := test.LoadEnvelope("invoice-ss.json")
		orig, err := tc.Convert(env)
		require.NoError(t, err)

		_, err = tc.GenerateZuzendu(env, orig, ticketbai.ZuzenduCorrect)
		assert.ErrorIs(t, err, ticketbai.ErrValidation)
		assert.ErrorContains(t, err, "original document must be signed")
	})

	t.Run("rejects an unknown action", func(t *testing.T) {
		orig, _ := issueOriginal(t)
		env := test.LoadEnvelope("invoice-ss.json")

		_, err := tc.GenerateZuzendu(env, orig, convert.ZuzenduAction("BORRAR"))
		assert.ErrorIs(t, err, ticketbai.ErrValidation)
	})

	t.Run("corrects a rejected cancellation", func(t *testing.T) {
		env := test.LoadEnvelope("invoice-ss.json")
		td, err := tc.Convert(env)
		require.NoError(t, err)
		require.NoError(t, tc.Fingerprint(td, nil))
		require.NoError(t, tc.Sign(td, env))

		cd, err := tc.GenerateCancel(env)
		require.NoError(t, err)
		require.NoError(t, tc.FingerprintCancel(cd))
		require.NoError(t, tc.SignCancel(cd, env))
		data, err := cd.Bytes()
		require.NoError(t, err)
		orig, err := ticketbai.ParseCancelDocument(data)
		require.NoError(t, err)

		zd, err := tc.GenerateZuzenduCancel(orig)
		require.NoError(t, err)
		assert.Equal(t, orig.IDFactura, zd.IDFactura)
		assert.Equal(t, orig.SignatureValue()[:100], zd.SignatureValueFirmaAnulacion)

		data, err = zd.BytesIndent()
		require.NoError(t, err)
		require.NoError(t, validateDoc(schema, data))
	})

	t.Run("sends through the connection", func(t *testing.T) {
		conn := new(ticketbai.TestConnection)
		c, err := ticketbai.New(&ticketbai.Software{}, ticketbai.ZoneSS, ticketbai.WithConnection(conn))
		require.NoError(t, err)
		env := test.LoadEnvelope("invoice-ss.json")
		ctx := context.Background()

		require.NoError(t, c.PostZuzendu(ctx, env, new(convert.ZuzenduAlta)))
		require.NoError(t, c.CancelZuzendu(ctx, env, new(convert.ZuzenduAnula)))

		conn.PostZuzenduError = gateways.ErrDuplicate
		conn.CancelZuzenduError = gateways.ErrValidation
		assert.ErrorIs(t, c.PostZuzendu(ctx, env, new(convert.ZuzenduAlta)), ticketbai.ErrDuplicate)
		assert.ErrorIs(t, c.CancelZuzendu(ctx, env, new(convert.ZuzenduAnula)), ticketbai.ErrValidation)
	})
}

func loadTBAIClientAt(ts time.Time) (*ticketbai.Client, error) {
	tc, err := loadTBAIClient()
	if err != nil {
		return nil, err
	}
	ticketbai.WithCurrentTime(ts)(tc)
	return tc, nil
}
