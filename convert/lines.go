package convert

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

var regime = tax.RegimeDefFor("ES")

const (
	// currencyDecimals is the number of decimals expected of a currency amount,
	// and the most an `ImporteSgn12.2Type` field accepts.
	currencyDecimals uint32 = 2
	// maxAmountDecimals is the most an `ImporteSgn12.8Type` field accepts.
	maxAmountDecimals uint32 = 8
)

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
		unit, discount := newImporteUnitarioDescuento(line)
		lines = append(lines, IDDetalleFactura{
			DescripcionDetalle: line.Item.Name,
			Cantidad:           line.Quantity.String(),
			ImporteUnitario:    unit,
			Descuento:          discount,
			ImporteTotal:       calculateTotal(line).Rescale(currencyDecimals).String(),
		})
	}

	return &DetallesFactura{
		IDDetalleFactura: lines,
	}
}

// newImporteUnitarioDescuento renders the tax-excluded unit price and discount
// of a line, which the gateway uses to derive the line total as
// `(ImporteUnitario x Cantidad - Descuento) x (1 + tipo)`.
//
// `ImporteUnitario` is an `ImporteSgn12.8Type` and the specification asks for
// as many decimals as are available, so it is only rounded up to the two
// decimals expected of a currency, never down to them: removing included taxes
// leaves the price with more precision than the currency (a 15.00 price with
// 21% VAT becomes 12.39669), and discarding it would leave the line total
// unable to be derived. `Descuento` is an `ImporteSgn12.2Type` in the v1.2
// schema this document declares, so it is held to two decimals.
//
// TicketBAI has no per-line charge. Charges are reported by reducing the
// discount, which `Sum - Total` does on its own, but a line whose charges
// outweigh its discounts would report a discount running against the line: a
// surcharge rather than the discount the field is defined as. Such a charge is
// folded into the unit price instead. Either arrangement satisfies the
// gateway's arithmetic; only this one keeps both fields meaning what they say.
//
// A discount is "against the line" when its sign opposes the line total's, not
// simply when it is negative: credit notes are inverted before conversion, so
// every amount on them, discounts included, is negative already.
func newImporteUnitarioDescuento(line *bill.Line) (unit, discount string) {
	price := *line.Item.Price
	amount := line.Sum.Subtract(*line.Total)

	if isSurcharge(amount, *line.Total) && !line.Quantity.IsZero() {
		price = line.Total.Rescale(maxAmountDecimals).Divide(line.Quantity)
		amount = num.AmountZero
	}

	return price.RescaleRange(currencyDecimals, maxAmountDecimals).String(),
		amount.Rescale(currencyDecimals).String()
}

// isSurcharge reports whether a line's discount runs against the line total,
// which happens when the line's charges outweigh its discounts.
func isSurcharge(discount, total num.Amount) bool {
	return !discount.IsZero() && discount.IsNegative() != total.IsNegative()
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
