package convert_test

import (
	"testing"
	"time"

	"github.com/invopop/gobl.ticketbai/convert"
	"github.com/invopop/gobl.ticketbai/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/es"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLines(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2022-02-01T04:00:00Z")
	require.NoError(t, err)
	role := convert.IssuerRoleThirdParty

	t.Run("should show line info", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("sample-invoice.json")
		goblInvoice.Lines = []*bill.Line{{
			Index:    1,
			Quantity: num.MakeAmount(100, 0),
			Item:     &org.Item{Name: "A", Price: num.NewAmount(10, 0)},
			Taxes:    tax.Set{&tax.Combo{Category: tax.CategoryVAT, Rate: "standard"}},
		}}
		_ = goblInvoice.Calculate()

		invoice, _ := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)

		lines := invoice.Factura.DatosFactura.DetallesFactura.IDDetalleFactura
		assert.Equal(t, 1, len(lines))
		assert.Equal(t, "A", lines[0].DescripcionDetalle)
		assert.Equal(t, "100", lines[0].Cantidad)
		assert.Equal(t, "10.00", lines[0].ImporteUnitario)
		assert.Equal(t, "1210.00", lines[0].ImporteTotal)
	})

	t.Run("should show line discount", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("sample-invoice.json")
		goblInvoice.Lines = []*bill.Line{{
			Index:     1,
			Quantity:  num.MakeAmount(100, 0),
			Item:      &org.Item{Name: "A", Price: num.NewAmount(11, 0)},
			Discounts: []*bill.LineDiscount{DiscountOf(100)},
			Taxes:     tax.Set{&tax.Combo{Category: tax.CategoryVAT, Rate: "standard"}},
		}}
		_ = goblInvoice.Calculate()

		invoice, _ := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)

		line := invoice.Factura.DatosFactura.DetallesFactura.IDDetalleFactura[0]
		assert.Equal(t, "100.00", line.Descuento)
		assert.Equal(t, "1210.00", line.ImporteTotal)
	})

	t.Run("should subtract taxes if included in prices per unit", func(t *testing.T) {
		inv := test.LoadInvoice("sample-invoice.json")
		inv.Tax = &bill.Tax{PricesInclude: "VAT"}

		inv.Lines = []*bill.Line{{
			Index:    1,
			Quantity: num.MakeAmount(10, 0),
			Item:     &org.Item{Name: "A", Price: num.NewAmount(121, 0)},
			Taxes:    tax.Set{&tax.Combo{Category: tax.CategoryVAT, Rate: "standard"}},
		}}
		require.NoError(t, inv.Calculate())
		require.NoError(t, inv.RemoveIncludedTaxes())

		out, _ := convert.NewTicketBAI(inv, ts, role, convert.ZoneBI)

		line := out.Factura.DatosFactura.DetallesFactura.IDDetalleFactura[0]
		assert.Equal(t, "100.00", line.ImporteUnitario)
		assert.Equal(t, "1210.00", line.ImporteTotal)
	})

	t.Run("should exclude retained taxes (IRPF) from line ImporteTotal", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("sample-invoice.json")
		goblInvoice.Lines = []*bill.Line{{
			Index:    1,
			Quantity: num.MakeAmount(100, 0),
			Item:     &org.Item{Name: "A", Price: num.NewAmount(10, 0)},
			Taxes: tax.Set{
				&tax.Combo{Category: tax.CategoryVAT, Rate: "standard"},
				&tax.Combo{Category: es.TaxCategoryIRPF, Rate: "pro"},
			},
		}}
		_ = goblInvoice.Calculate()

		invoice, _ := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)

		line := invoice.Factura.DatosFactura.DetallesFactura.IDDetalleFactura[0]
		assert.Equal(t, "1210.00", line.ImporteTotal)
		assert.Equal(t, "1210.00", invoice.Factura.DatosFactura.ImporteTotalFactura)
	})

	t.Run("should not round line taxes to whole units when prices include tax", func(t *testing.T) {
		// Removing included taxes leaves line totals with more decimal places
		// than the currency, so the per-line tax must be accumulated at that
		// same precision. Previously the tax was rounded to whole euros, which
		// left ImporteTotal wildly wrong and unable to add up to
		// ImporteTotalFactura.
		inv := test.LoadInvoice("invoice-es-es-tbai-simplified.json")
		inv.Tax.PricesInclude = "VAT"
		inv.Lines = []*bill.Line{
			{
				Index:    1,
				Quantity: num.MakeAmount(1, 0),
				Item:     &org.Item{Name: "Room", Price: num.NewAmount(15000, 3)},
				Taxes:    tax.Set{&tax.Combo{Category: tax.CategoryVAT, Rate: "standard"}},
			},
			{
				Index:    2,
				Quantity: num.MakeAmount(1, 0),
				Item:     &org.Item{Name: "Fees", Price: num.NewAmount(1200, 3)},
				Taxes:    tax.Set{&tax.Combo{Category: tax.CategoryVAT, Rate: "standard"}},
			},
		}
		require.NoError(t, inv.Calculate())
		require.NoError(t, inv.RemoveIncludedTaxes())

		out, err := convert.NewTicketBAI(inv, ts, role, convert.ZoneSS)
		require.NoError(t, err)

		datos := out.Factura.DatosFactura
		lines := datos.DetallesFactura.IDDetalleFactura
		require.Len(t, lines, 2)

		assert.Equal(t, "12.40", lines[0].ImporteUnitario)
		assert.Equal(t, "15.00", lines[0].ImporteTotal)
		assert.Equal(t, "0.99", lines[1].ImporteUnitario)
		assert.Equal(t, "1.20", lines[1].ImporteTotal)
		assert.Equal(t, "16.20", datos.ImporteTotalFactura)

		// The reported line totals must add up to the invoice total.
		sum := num.AmountZero
		for _, line := range lines {
			amount, err := num.AmountFromString(line.ImporteTotal)
			require.NoError(t, err)
			sum = sum.MatchPrecision(amount).Add(amount)
		}
		assert.Equal(t, datos.ImporteTotalFactura, sum.Rescale(2).String())
	})

	t.Run("should limit the discount to two decimal places", func(t *testing.T) {
		// TicketBAI v1.2 declares Descuento as ImporteSgn12.2Type, but removing
		// included taxes adds decimal places to the line amounts.
		inv := test.LoadInvoice("invoice-es-es-tbai-simplified.json")
		inv.Tax.PricesInclude = "VAT"
		inv.Lines = []*bill.Line{{
			Index:     1,
			Quantity:  num.MakeAmount(1, 0),
			Item:      &org.Item{Name: "Room", Price: num.NewAmount(15000, 3)},
			Discounts: []*bill.LineDiscount{DiscountOf(1)},
			Taxes:     tax.Set{&tax.Combo{Category: tax.CategoryVAT, Rate: "standard"}},
		}}
		require.NoError(t, inv.Calculate())
		require.NoError(t, inv.RemoveIncludedTaxes())

		out, err := convert.NewTicketBAI(inv, ts, role, convert.ZoneSS)
		require.NoError(t, err)

		line := out.Factura.DatosFactura.DetallesFactura.IDDetalleFactura[0]
		assert.Equal(t, "0.83", line.Descuento)
	})

	t.Run("should return error if more than 1000 lines included and not Vizcaya", func(t *testing.T) {
		inv := test.LoadInvoice("sample-invoice.json")
		inv.Lines = []*bill.Line{}
		for i := 1; i <= 1001; i++ {
			inv.Lines = append(inv.Lines, &bill.Line{
				Index:    1,
				Quantity: num.MakeAmount(100, 0),
				Item:     &org.Item{Name: "A", Price: num.NewAmount(10, 0)},
				Taxes:    tax.Set{&tax.Combo{Category: tax.CategoryVAT, Rate: tax.RateGeneral}},
			})
		}
		require.NoError(t, inv.Calculate())

		_, err := convert.NewTicketBAI(inv, ts, role, convert.ZoneSS)

		assert.ErrorContains(t, err, "line count over limit (1000) for tax locality")
	})
}
