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
	"github.com/invopop/gobl/l10n"
	"github.com/lestrrat-go/helium/xsd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAmendment(t *testing.T) {
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

		ad, err := later.GenerateAmendment(env, orig, ticketbai.AmendReceived)
		require.NoError(t, err)

		assert.Equal(t, convert.AmendReceived, ad.Cabecera.Accion)
		assert.Equal(t, "Corrected Name SL", ad.Sujetos.Destinatarios.IDDestinatario[0].ApellidosNombreRazonSocial)
		assert.Equal(t, orig.Head().FechaExpedicionFactura, ad.Factura.CabeceraFactura.FechaExpedicionFactura)
		assert.Equal(t, orig.Head().HoraExpedicionFactura, ad.Factura.CabeceraFactura.HoraExpedicionFactura)
		assert.Equal(t, orig.HuellaTBAI, ad.HuellaTBAI)
		assert.Len(t, ad.SignatureValueFirmaFactura, 100)
		assert.Equal(t, orig.SignatureValue()[:100], ad.SignatureValueFirmaFactura)

		data, err := ad.BytesIndent()
		require.NoError(t, err)
		require.NoError(t, validateDoc(schema, data))
	})

	t.Run("corrects a rejected invoice", func(t *testing.T) {
		orig, stamps := issueOriginal(t)
		env := test.LoadEnvelope("invoice-ss.json")

		ad, err := tc.GenerateAmendment(env, orig, ticketbai.AmendRejected)
		require.NoError(t, err)
		assert.Equal(t, convert.AmendRejected, ad.Cabecera.Accion)

		data, err := ad.Bytes()
		require.NoError(t, err)
		require.NoError(t, validateDoc(schema, data))

		// The TicketBAI code and QR come from the original signature, so
		// they remain valid for the corrected document.
		assert.Contains(t, stamps[0], orig.SignatureValue()[:13])

		parsed, err := ticketbai.ParseAmendmentDocument(data)
		require.NoError(t, err)
		assert.Equal(t, ad.Cabecera, parsed.Cabecera)
		assert.Equal(t, ad.Factura.CabeceraFactura, parsed.Factura.CabeceraFactura)
		assert.Equal(t, ad.HuellaTBAI, parsed.HuellaTBAI)
		assert.Equal(t, ad.SignatureValueFirmaFactura, parsed.SignatureValueFirmaFactura)
	})

	t.Run("rejects a different invoice", func(t *testing.T) {
		orig, _ := issueOriginal(t)
		env := test.LoadEnvelope("invoice-ss.json")
		env.Extract().(*bill.Invoice).Code = cbc.Code("OTHER")

		_, err := tc.GenerateAmendment(env, orig, ticketbai.AmendReceived)
		assert.ErrorIs(t, err, ticketbai.ErrValidation)
		assert.ErrorContains(t, err, "invoice code does not match")
	})

	t.Run("rejects a different supplier", func(t *testing.T) {
		orig, _ := issueOriginal(t)
		env := test.LoadEnvelope("invoice-ss.json")
		env.Extract().(*bill.Invoice).Supplier.TaxID.Code = cbc.Code("B85905495")

		_, err := tc.GenerateAmendment(env, orig, ticketbai.AmendReceived)
		assert.ErrorIs(t, err, ticketbai.ErrValidation)
		assert.ErrorContains(t, err, "supplier does not match")
	})

	t.Run("rejects an unsigned original", func(t *testing.T) {
		env := test.LoadEnvelope("invoice-ss.json")
		orig, err := tc.Convert(env)
		require.NoError(t, err)

		_, err = tc.GenerateAmendment(env, orig, ticketbai.AmendRejected)
		assert.ErrorIs(t, err, ticketbai.ErrValidation)
		assert.ErrorContains(t, err, "original document must be signed")
	})

	t.Run("rejects an unknown action", func(t *testing.T) {
		orig, _ := issueOriginal(t)
		env := test.LoadEnvelope("invoice-ss.json")

		_, err := tc.GenerateAmendment(env, orig, convert.AmendmentAction("BORRAR"))
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

		ad, err := tc.GenerateCancelAmendment(orig)
		require.NoError(t, err)
		assert.Equal(t, orig.IDFactura, ad.IDFactura)
		assert.Equal(t, orig.SignatureValue()[:100], ad.SignatureValueFirmaAnulacion)

		data, err = ad.BytesIndent()
		require.NoError(t, err)
		require.NoError(t, validateDoc(schema, data))
	})

	t.Run("sends through the connection", func(t *testing.T) {
		conn := new(ticketbai.TestConnection)
		c, err := ticketbai.New(&ticketbai.Software{}, ticketbai.ZoneSS, ticketbai.WithConnection(conn))
		require.NoError(t, err)
		env := test.LoadEnvelope("invoice-ss.json")
		ctx := context.Background()

		require.NoError(t, c.PostAmendment(ctx, env, new(convert.Amendment)))
		require.NoError(t, c.CancelAmendment(ctx, env, new(convert.CancelAmendment)))

		conn.PostAmendmentError = gateways.ErrDuplicate
		conn.CancelAmendmentError = gateways.ErrValidation
		assert.ErrorIs(t, c.PostAmendment(ctx, env, new(convert.Amendment)), ticketbai.ErrDuplicate)
		assert.ErrorIs(t, c.CancelAmendment(ctx, env, new(convert.CancelAmendment)), ticketbai.ErrValidation)
	})
}

func TestSupportsAmendments(t *testing.T) {
	for zone, supported := range map[l10n.Code]bool{
		ticketbai.ZoneBI: false,
		ticketbai.ZoneSS: true,
		ticketbai.ZoneVI: true,
	} {
		c, err := ticketbai.New(&ticketbai.Software{}, zone, ticketbai.WithConnection(new(ticketbai.TestConnection)))
		require.NoError(t, err)
		assert.Equal(t, supported, c.SupportsAmendments(), zone)
	}
}

func TestPostAmendmentFallback(t *testing.T) {
	env := test.LoadEnvelope("invoice-ss.json")
	ctx := context.Background()
	newAmendment := func(action convert.AmendmentAction) *convert.Amendment {
		return &convert.Amendment{Cabecera: &convert.AmendmentCabecera{IDVersion: "1.0", Accion: action}}
	}

	t.Run("modifies when the original was registered", func(t *testing.T) {
		conn := &sequenceConnection{errs: []error{gateways.ErrDuplicate, nil}}
		c, err := ticketbai.New(&ticketbai.Software{}, ticketbai.ZoneSS, ticketbai.WithConnection(conn))
		require.NoError(t, err)

		ad := newAmendment(ticketbai.AmendRejected)
		require.NoError(t, c.PostAmendment(ctx, env, ad))
		assert.Equal(t, []convert.AmendmentAction{ticketbai.AmendRejected, ticketbai.AmendReceived}, conn.actions)
		assert.Equal(t, ticketbai.AmendReceived, ad.Cabecera.Accion)
	})

	t.Run("reports errors from the modification", func(t *testing.T) {
		conn := &sequenceConnection{errs: []error{gateways.ErrDuplicate, gateways.ErrValidation}}
		c, err := ticketbai.New(&ticketbai.Software{}, ticketbai.ZoneSS, ticketbai.WithConnection(conn))
		require.NoError(t, err)

		err = c.PostAmendment(ctx, env, newAmendment(ticketbai.AmendRejected))
		assert.ErrorIs(t, err, ticketbai.ErrValidation)
		assert.Len(t, conn.actions, 2)
	})

	t.Run("does not retry other errors", func(t *testing.T) {
		conn := &sequenceConnection{errs: []error{gateways.ErrValidation}}
		c, err := ticketbai.New(&ticketbai.Software{}, ticketbai.ZoneSS, ticketbai.WithConnection(conn))
		require.NoError(t, err)

		err = c.PostAmendment(ctx, env, newAmendment(ticketbai.AmendRejected))
		assert.ErrorIs(t, err, ticketbai.ErrValidation)
		assert.Len(t, conn.actions, 1)
	})

	t.Run("does not retry modifications", func(t *testing.T) {
		conn := &sequenceConnection{errs: []error{gateways.ErrDuplicate}}
		c, err := ticketbai.New(&ticketbai.Software{}, ticketbai.ZoneSS, ticketbai.WithConnection(conn))
		require.NoError(t, err)

		err = c.PostAmendment(ctx, env, newAmendment(ticketbai.AmendReceived))
		assert.ErrorIs(t, err, ticketbai.ErrDuplicate)
		assert.Len(t, conn.actions, 1)
	})
}

// sequenceConnection returns the given errors in order from PostAmendment,
// and records the actions it was sent.
type sequenceConnection struct {
	ticketbai.TestConnection
	errs    []error
	actions []convert.AmendmentAction
}

func (c *sequenceConnection) PostAmendment(_ context.Context, _ *bill.Invoice, d *convert.Amendment) error {
	c.actions = append(c.actions, d.Cabecera.Accion)
	err := c.errs[0]
	c.errs = c.errs[1:]
	return err
}

func loadTBAIClientAt(ts time.Time) (*ticketbai.Client, error) {
	tc, err := loadTBAIClient()
	if err != nil {
		return nil, err
	}
	ticketbai.WithCurrentTime(ts)(tc)
	return tc, nil
}
