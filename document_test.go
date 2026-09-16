package ticketbai_test

import (
	"testing"

	"github.com/invopop/gobl/bill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/invopop/gobl.ticketbai/test"
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
