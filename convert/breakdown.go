package convert

import (
	"strings"

	"github.com/invopop/gobl/addons/es/tbai"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

const SubjectAndNotExemptCode = "S1"

// TipoDesglose contains info about the taxes breakdown of
// an invoice
type TipoDesglose struct {
	DesgloseFactura       *DesgloseFactura
	DesgloseTipoOperacion *DesgloseTipoOperacion
}

// DesgloseFactura contains taxes breakdown if customer is from Spain or there
// is no customer
type DesgloseFactura struct {
	Sujeta   *Sujeta
	NoSujeta *NoSujeta
}

// DesgloseTipoOperacion contains taxes breakdown if customer is from abroad
type DesgloseTipoOperacion struct {
	PrestacionServicios *DesgloseFactura
	Entrega             *DesgloseFactura
}

// Sujeta means that some amount is liable to taxes (VAT, equivalence surcharge)
type Sujeta struct {
	Exenta   *Exenta
	NoExenta *NoExenta
}

// Exenta contains the info of the amounts that even being liable to taxes, are
// under 0% rate due to different reasons
type Exenta struct {
	DetalleExenta []*DetalleExenta
}

// DetalleExenta details about 0% taxed amounts
type DetalleExenta struct {
	CausaExencion string
	BaseImponible string
}

// NoExenta list the amounts that a liable to taxes (VAT, equivalence surcharge) other than 0%
type NoExenta struct {
	DetalleNoExenta []*DetalleNoExenta
}

// DetalleNoExenta details about non 0% taxes amounts
type DetalleNoExenta struct {
	TipoNoExenta string
	DesgloseIVA  *DesgloseIVA
}

// DesgloseIVA list of VAT details
type DesgloseIVA struct {
	DetalleIVA []*DetalleIVA
}

// DetalleIVA contains details of VAT / equivalence surcharge taxes
type DetalleIVA struct {
	BaseImponible                                        string
	TipoImpositivo                                       string
	CuotaImpuesto                                        string
	TipoRecargoEquivalencia                              string `xml:",omitempty"`
	CuotaRecargoEquivalencia                             string `xml:",omitempty"`
	OperacionEnRecargoDeEquivalenciaORegimenSimplificado string `xml:",omitempty"`
}

// NoSujeta means that some part of the invoice is not liable to VAT
type NoSujeta struct {
	DetalleNoSujeta []*DetalleNoSujeta
}

// DetalleNoSujeta contains details about the not liable amount
type DetalleNoSujeta struct {
	Causa   string
	Importe num.Amount
}

func newTipoDesglose(gobl *bill.Invoice) *TipoDesglose {
	if gobl.Totals == nil || gobl.Totals.Taxes == nil {
		return nil
	}
	catTotal := gobl.Totals.Taxes.Category(tax.CategoryVAT)
	if catTotal == nil {
		return nil
	}

	desglose := &TipoDesglose{}

	if gobl.Customer == nil || partyCountry(gobl.Customer) == l10n.ES.Tax() || isSimplified(gobl) {
		desglose.DesgloseFactura = newDesgloseFactura(catTotal.Rates)
	} else {
		goods, services := splitByTBAIProduct(catTotal.Rates)

		desglose.DesgloseTipoOperacion = &DesgloseTipoOperacion{
			Entrega:             newDesgloseFactura(goods),
			PrestacionServicios: newDesgloseFactura(services),
		}
	}

	return desglose
}

func newDesgloseFactura(rates []*tax.RateTotal) *DesgloseFactura {
	if len(rates) == 0 {
		return nil
	}

	df := &DesgloseFactura{
		NoSujeta: &NoSujeta{},
		Sujeta: &Sujeta{
			Exenta:   &Exenta{},
			NoExenta: &NoExenta{},
		},
	}

	for _, rate := range rates {
		code := rate.Ext.Get(tbai.ExtKeyExempt)
		switch {
		case code.In(notSubjectExemptionCodes...):
			df.NoSujeta.appendDetalle(&DetalleNoSujeta{
				Causa:   code.String(),
				Importe: rate.Base,
			})
		case code.In(exemptExemptionCodes...):
			df.Sujeta.Exenta.appendDetalle(&DetalleExenta{
				CausaExencion: code.String(),
				BaseImponible: rate.Base.Rescale(2).String(),
			})
		default:
			if code.IsEmpty() {
				code = cbc.Code(SubjectAndNotExemptCode)
			}
			dne := df.Sujeta.NoExenta.appendDetalle(&DetalleNoExenta{
				TipoNoExenta: code.String(),
				DesgloseIVA:  &DesgloseIVA{},
			})
			dne.DesgloseIVA.appendDetalle(newDetalleIVA(rate))
		}
	}

	return df.prune()
}

func splitByTBAIProduct(rates []*tax.RateTotal) (goods, services []*tax.RateTotal) {
	for _, rate := range rates {
		if rate.Ext.Get(tbai.ExtKeyProduct) == tbai.ExtValueProductGoods {
			goods = append(goods, rate)
		} else {
			services = append(services, rate)
		}
	}

	return goods, services
}

func (df *DesgloseFactura) prune() *DesgloseFactura {
	if len(df.NoSujeta.DetalleNoSujeta) == 0 {
		df.NoSujeta = nil
	}
	if len(df.Sujeta.Exenta.DetalleExenta) == 0 {
		df.Sujeta.Exenta = nil
	}
	if len(df.Sujeta.NoExenta.DetalleNoExenta) == 0 {
		df.Sujeta.NoExenta = nil
	}
	if df.Sujeta.Exenta == nil && df.Sujeta.NoExenta == nil {
		df.Sujeta = nil
	}

	return df
}

func (ns *NoSujeta) appendDetalle(d *DetalleNoSujeta) *DetalleNoSujeta {
	for _, e := range ns.DetalleNoSujeta {
		if e.Causa == d.Causa {
			e.Importe = e.Importe.Add(d.Importe)
			return e
		}
	}
	ns.DetalleNoSujeta = append(ns.DetalleNoSujeta, d)
	return d
}

func (e *Exenta) appendDetalle(d *DetalleExenta) *DetalleExenta {
	e.DetalleExenta = append(e.DetalleExenta, d)
	return d
}

func (ne *NoExenta) appendDetalle(d *DetalleNoExenta) *DetalleNoExenta {
	for _, e := range ne.DetalleNoExenta {
		if e.TipoNoExenta == d.TipoNoExenta {
			return e
		}
	}
	ne.DetalleNoExenta = append(ne.DetalleNoExenta, d)
	return d
}

func (di *DesgloseIVA) appendDetalle(d *DetalleIVA) *DetalleIVA {
	di.DetalleIVA = append(di.DetalleIVA, d)
	return d
}

func newDetalleIVA(rate *tax.RateTotal) *DetalleIVA {
	percent := num.PercentageZero
	if rate.Percent != nil {
		percent = *rate.Percent
	}
	diva := &DetalleIVA{
		BaseImponible:  rate.Base.Rescale(2).String(),
		TipoImpositivo: formatPercent(percent),
		CuotaImpuesto:  rate.Amount.Rescale(2).String(),
	}

	if rate.Surcharge != nil {
		diva.TipoRecargoEquivalencia = formatPercent(rate.Surcharge.Percent)
		diva.CuotaRecargoEquivalencia = rate.Surcharge.Amount.Rescale(2).String()
	}

	if rate.Ext.Get(tbai.ExtKeyRegime) == "52" || rate.Ext.Get(tbai.ExtKeyProduct) == tbai.ExtValueProductResale {
		diva.OperacionEnRecargoDeEquivalenciaORegimenSimplificado = "S"
	}

	return diva
}

func formatPercent(percent num.Percentage) string {
	maybeNegative := percent.Amount().Rescale(2).String()
	if strings.Contains(maybeNegative, "-") {
		return strings.ReplaceAll(maybeNegative, "-", "")
	}

	return maybeNegative
}

// es-tbai-exemption codes routed to DetalleNoSujeta.
var notSubjectExemptionCodes = []cbc.Code{"OT", "RL", "VT", "IE"}

// es-tbai-exemption codes routed to Sujeta.Exenta.
var exemptExemptionCodes = []cbc.Code{"E1", "E2", "E3", "E4", "E5", "E6"}
