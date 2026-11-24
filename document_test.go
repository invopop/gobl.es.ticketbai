package ticketbai_test

import (
	"testing"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/es"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/invopop/gobl.es.ticketbai/test"
)

func TestConvertRemovesIncludedTaxes(t *testing.T) {
	tbai, err := loadTBAIClient()
	require.NoError(t, err)

	t.Run("removes taxes included in the prices", func(t *testing.T) {
		env := test.LoadEnvelope("invoice-es-es-tbai-prices-included.json")

		doc, err := tbai.Convert(env)
		require.NoError(t, err)

		lines := doc.Factura.DatosFactura.DetallesFactura.IDDetalleFactura
		require.Len(t, lines, 2)
		assert.Equal(t, "12.39669", lines[0].ImporteUnitario)
		assert.Equal(t, "15.00", lines[0].ImporteTotal)
		assert.Equal(t, "0.99174", lines[1].ImporteUnitario)
		assert.Equal(t, "1.20", lines[1].ImporteTotal)
		assert.Equal(t, "16.20", doc.Factura.DatosFactura.ImporteTotalFactura)
	})

	t.Run("is a no-op when the taxes were already removed upstream", func(t *testing.T) {
		// The silo can strip included taxes as it serves an entry, depending on
		// how the app action is configured. Conversion has to land in the same
		// place whether or not it did.
		raw := test.LoadEnvelope("invoice-es-es-tbai-prices-included.json")
		rawDoc, err := tbai.Convert(raw)
		require.NoError(t, err)

		pre := test.LoadEnvelope("invoice-es-es-tbai-prices-included.json")
		inv, ok := pre.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NoError(t, inv.RemoveIncludedTaxes())
		require.NoError(t, pre.Insert(inv))

		preDoc, err := tbai.Convert(pre)
		require.NoError(t, err)

		assert.Equal(t, rawDoc.Factura.DatosFactura, preDoc.Factura.DatosFactura)
		assert.Equal(t, rawDoc.Factura.TipoDesglose, preDoc.Factura.TipoDesglose)
	})

	t.Run("leaves the caller's envelope untouched", func(t *testing.T) {
		// The envelope is signed by the time it reaches us and its digest
		// covers the document, so removing the included taxes must happen on a
		// copy.
		env := test.LoadEnvelope("invoice-es-es-tbai-prices-included.json")

		_, err := tbai.Convert(env)
		require.NoError(t, err)

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		assert.Equal(t, "VAT", inv.Tax.PricesInclude.String())
		assert.Equal(t, "15.000", inv.Lines[0].Item.Price.String())
		assert.Equal(t, "16.20", inv.Totals.Sum.String())
		require.NoError(t, env.Validate(), "envelope digest must still match")
	})
}

func TestConvertCreditNotes(t *testing.T) {
	tbai, err := loadTBAIClient()
	require.NoError(t, err)

	t.Run("negates the amounts", func(t *testing.T) {
		env := test.LoadEnvelope("credit-note-es-es-tbai.json")

		doc, err := tbai.Convert(env)
		require.NoError(t, err)

		lines := doc.Factura.DatosFactura.DetallesFactura.IDDetalleFactura
		require.Len(t, lines, 2)
		assert.Equal(t, "-20", lines[0].Cantidad)
		assert.Equal(t, "90.00", lines[0].ImporteUnitario)
		assert.Equal(t, "-180.00", lines[0].Descuento)
		assert.Equal(t, "-1960.20", lines[0].ImporteTotal)
		assert.Equal(t, "-1965.20", doc.Factura.DatosFactura.ImporteTotalFactura)
		iva := doc.Factura.TipoDesglose.DesgloseFactura.Sujeta.NoExenta.DetalleNoExenta[0].DesgloseIVA.DetalleIVA[0]
		assert.Equal(t, "-1620.00", iva.BaseImponible)
		assert.Equal(t, "21.00", iva.TipoImpositivo)
		assert.Equal(t, "-340.20", iva.CuotaImpuesto)
	})

	t.Run("leaves the caller's envelope untouched", func(t *testing.T) {
		env := test.LoadEnvelope("credit-note-es-es-tbai.json")

		_, err := tbai.Convert(env)
		require.NoError(t, err)

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		assert.Equal(t, "20", inv.Lines[0].Quantity.String())
		assert.Equal(t, "1965.20", inv.Totals.Payable.String())
		require.NoError(t, env.Validate(), "envelope digest must still match")
	})

	t.Run("uses the totals provided with the bypass tag", func(t *testing.T) {
		env := test.LoadEnvelope("credit-note-es-es-tbai.json")
		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		inv.SetTags(tax.TagBypass)
		// Totals that would not survive a recalculation
		cat := inv.Totals.Taxes.Categories[0]
		cat.Rates[0].Amount = num.MakeAmount(34021, 2)
		cat.Amount = cat.Rates[0].Amount
		inv.Totals.Taxes.Sum = cat.Amount
		inv.Totals.Tax = cat.Amount
		inv.Totals.TotalWithTax = num.MakeAmount(196521, 2)
		inv.Totals.Payable = inv.Totals.TotalWithTax

		doc, err := tbai.Convert(env)
		require.NoError(t, err)

		assert.Equal(t, "-1965.21", doc.Factura.DatosFactura.ImporteTotalFactura)
		iva := doc.Factura.TipoDesglose.DesgloseFactura.Sujeta.NoExenta.DetalleNoExenta[0].DesgloseIVA.DetalleIVA[0]
		assert.Equal(t, "-340.21", iva.CuotaImpuesto)
		assert.Equal(t, "1965.21", inv.Totals.Payable.String())
	})

	t.Run("keeps the rounding provided", func(t *testing.T) {
		env := test.LoadEnvelope("credit-note-es-es-tbai.json")
		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		rounding := num.MakeAmount(-2, 2)
		inv.Totals.Rounding = &rounding
		require.NoError(t, env.Calculate())

		doc, err := tbai.Convert(env)
		require.NoError(t, err)

		assert.Equal(t, "-1965.18", doc.Factura.DatosFactura.ImporteTotalFactura)
	})

	// creditNoteWith loads the sample credit note with a single 500.00 line at
	// the sample's VAT rate plus 15% IRPF, adjusted by setup.
	creditNoteWith := func(t *testing.T, setup func(line *bill.Line)) *gobl.Envelope {
		t.Helper()
		env := test.LoadEnvelope("credit-note-es-es-tbai.json")
		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)

		// Replace the lines with one line using the sample's VAT combo
		taxes := tax.Set{inv.Lines[0].Taxes[0], &tax.Combo{Category: es.TaxCategoryIRPF, Percent: num.NewPercentage(150, 3)}}
		line := &bill.Line{
			Quantity: num.MakeAmount(1, 0),
			Item:     &org.Item{Name: "Works", Price: num.NewAmount(50000, 2)},
			Taxes:    taxes,
		}
		setup(line)
		inv.Lines = []*bill.Line{line}

		// Recalculate the totals the credit note carries
		require.NoError(t, env.Calculate())
		return env
	}

	t.Run("negates a percent discount with a base equal to the line sum", func(t *testing.T) {
		env := creditNoteWith(t, func(line *bill.Line) {
			line.Discounts = []*bill.LineDiscount{{
				Reason:  "Works",
				Base:    num.NewAmount(50000, 2),
				Percent: num.NewPercentage(100, 3),
			}}
		})

		doc, err := tbai.Convert(env)
		require.NoError(t, err)

		lines := doc.Factura.DatosFactura.DetallesFactura.IDDetalleFactura
		require.Len(t, lines, 1)
		assert.Equal(t, "-1", lines[0].Cantidad)
		assert.Equal(t, "500.00", lines[0].ImporteUnitario)
		assert.Equal(t, "-50.00", lines[0].Descuento)
		assert.Equal(t, "-544.50", lines[0].ImporteTotal)
		assert.Equal(t, "-544.50", doc.Factura.DatosFactura.ImporteTotalFactura)
		assert.Equal(t, "-67.50", doc.Factura.DatosFactura.RetencionSoportada)
		iva := doc.Factura.TipoDesglose.DesgloseFactura.Sujeta.NoExenta.DetalleNoExenta[0].DesgloseIVA.DetalleIVA[0]
		assert.Equal(t, "-450.00", iva.BaseImponible)
		assert.Equal(t, "-94.50", iva.CuotaImpuesto)
	})

	t.Run("negates a percent discount with a base different from the line sum", func(t *testing.T) {
		env := creditNoteWith(t, func(line *bill.Line) {
			line.Discounts = []*bill.LineDiscount{{
				Reason:  "Works",
				Base:    num.NewAmount(40000, 2),
				Percent: num.NewPercentage(100, 3),
			}}
		})

		doc, err := tbai.Convert(env)
		require.NoError(t, err)

		lines := doc.Factura.DatosFactura.DetallesFactura.IDDetalleFactura
		require.Len(t, lines, 1)
		assert.Equal(t, "-40.00", lines[0].Descuento)
		assert.Equal(t, "-556.60", lines[0].ImporteTotal)
		assert.Equal(t, "-556.60", doc.Factura.DatosFactura.ImporteTotalFactura)
		assert.Equal(t, "-69.00", doc.Factura.DatosFactura.RetencionSoportada)
	})

	t.Run("negates a percent charge with a base", func(t *testing.T) {
		env := creditNoteWith(t, func(line *bill.Line) {
			line.Charges = []*bill.LineCharge{{
				Reason:  "Works",
				Base:    num.NewAmount(50000, 2),
				Percent: num.NewPercentage(100, 3),
			}}
		})

		doc, err := tbai.Convert(env)
		require.NoError(t, err)

		// The charge is folded into the unit price, as TicketBAI has no line charge
		lines := doc.Factura.DatosFactura.DetallesFactura.IDDetalleFactura
		require.Len(t, lines, 1)
		assert.Equal(t, "550.00000000", lines[0].ImporteUnitario)
		assert.Equal(t, "0.00", lines[0].Descuento)
		assert.Equal(t, "-665.50", lines[0].ImporteTotal)
		assert.Equal(t, "-665.50", doc.Factura.DatosFactura.ImporteTotalFactura)
		assert.Equal(t, "-82.50", doc.Factura.DatosFactura.RetencionSoportada)
	})
}
