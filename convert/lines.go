package convert

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

var regime = tax.RegimeDefFor("ES")

// DetallesFactura contains a list of detail lines info
type DetallesFactura struct {
	IDDetalleFactura []IDDetalleFactura
}

// IDDetalleFactura contains info about a detail line of the invoice
type IDDetalleFactura struct {
	DescripcionDetalle string
	Cantidad           string
	ImporteUnitario    string // Without VAT
	Descuento          string
	ImporteTotal       string
}

func newDetallesFactura(gobl *bill.Invoice) *DetallesFactura {
	lines := []IDDetalleFactura{}
	for _, line := range gobl.Lines {
		if line.Item.Price == nil {
			continue
		}
		lines = append(lines, IDDetalleFactura{
			DescripcionDetalle: line.Item.Name,
			Cantidad:           line.Quantity.String(),
			ImporteUnitario:    line.Item.Price.Rescale(2).String(),
			Descuento:          calculateDiscounts(line).String(),
			ImporteTotal:       calculateTotal(line).Rescale(2).String(),
		})
	}

	return &DetallesFactura{
		IDDetalleFactura: lines,
	}
}

// calculateDiscounts determines the per-line discount. Amounts are rescaled to
// two decimals as required by the TicketBAI `ImporteSgn12.2Type` used for the
// `Descuento` field; without this, invoices whose prices included tax would
// emit the extra decimal places added while removing it.
func calculateDiscounts(line *bill.Line) num.Amount {
	return line.Sum.Subtract(*line.Total).Rescale(2)
}

func calculateTotal(line *bill.Line) num.Amount {
	taxes := calculateTaxes(line)

	return line.Total.Add(taxes)
}

// calculateTaxes sums the non-retained taxes due on a line.
//
// The accumulator starts at zero with no decimal places, so every amount added
// to it must be matched to its precision first: `num.Amount.Add` rounds its
// argument to the receiver's exponent, which would otherwise round each tax
// amount to whole units.
func calculateTaxes(line *bill.Line) num.Amount {
	total := num.AmountZero
	for _, t := range line.Taxes {
		cat := regime.CategoryDef(t.Category)
		if cat == nil || cat.Retained {
			continue
		}
		if t.Percent != nil {
			amount := t.Percent.Of(*line.Total)
			total = total.MatchPrecision(amount).Add(amount)
		}
	}
	return total
}
