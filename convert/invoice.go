package convert

import (
	"github.com/invopop/gobl/addons/es/tbai"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// Factura contains the invoice info
type Factura struct {
	CabeceraFactura *CabeceraFactura
	DatosFactura    *DatosFactura
	TipoDesglose    *TipoDesglose
}

// CabeceraFactura contains info about the invoice header
type CabeceraFactura struct {
	SerieFactura                    string `xml:",omitempty"`
	NumFactura                      string
	FechaExpedicionFactura          string
	HoraExpedicionFactura           string
	FacturaSimplificada             string
	FacturaRectificativa            *FacturaRectificativa            `xml:",omitempty"`
	FacturasRectificadasSustituidas *FacturasRectificadasSustituidas `xml:",omitempty"`
}

// DatosFactura contains info about the invoice description
// and totals
type DatosFactura struct {
	FechaOperacion      string
	DescripcionFactura  string
	DetallesFactura     *DetallesFactura
	ImporteTotalFactura string
	RetencionSoportada  string
	Claves              *Claves
}

// Claves contains a list of keys (min 1, max 3) of VAT types
type Claves struct {
	IDClave []IDClave
}

// IDClave is the key of a single VAT type
type IDClave struct {
	ClaveRegimenIvaOpTrascendencia string
}

// FacturaRectificativa contains the info a corrective invoice
type FacturaRectificativa struct {
	Codigo string
	Tipo   string
}

// FacturasRectificadasSustituidas contains the info of all the invoices corrected or substituted in
// a corrective invoice
type FacturasRectificadasSustituidas struct {
	IDFacturaRectificadaSustituida []*IDFacturaRectificadaSustituida
}

// IDFacturaRectificadaSustituida contains the info of a single invoice corrected or substituted in
// a corrective invoice
type IDFacturaRectificadaSustituida struct {
	SerieFactura           string
	NumFactura             string
	FechaExpedicionFactura string
}

func newCabeceraFactura(inv *bill.Invoice) *CabeceraFactura {
	simplifiedInvoice := tbai.ExtValueSimplifiedNo.String()
	if isSimplified(inv) {
		simplifiedInvoice = tbai.ExtValueSimplifiedYes.String()
	}

	return &CabeceraFactura{
		SerieFactura:                    inv.Series.String(),
		NumFactura:                      inv.Code.String(),
		FacturaSimplificada:             simplifiedInvoice,
		FacturaRectificativa:            newFacturaRectificativa(inv),
		FacturasRectificadasSustituidas: newFacturasRectificadasSustituidas(inv),
	}
}

// isSimplified reports whether the invoice carries the es-tbai-simplified=S
// extension that the addon's normalizer sets from the GOBL simplified tag.
func isSimplified(inv *bill.Invoice) bool {
	return inv.Tax != nil && inv.Tax.Ext.Get(tbai.ExtKeySimplified) == tbai.ExtValueSimplifiedYes
}

func newDatosFactura(inv *bill.Invoice) (*DatosFactura, error) {
	description, err := newDescription(inv.Notes)
	if err != nil {
		return nil, err
	}

	// This is only needed on Guipuzcoa and Alava, but Vizcaya documentation
	// states that it will be safely ignored so it will be added for everyone
	lineDetails := newDetallesFactura(inv)

	opDate := inv.OperationDate
	if opDate == nil {
		opDate = &inv.IssueDate
	}
	opDateStr := formatDate(opDate)

	return &DatosFactura{
		FechaOperacion:      opDateStr,
		DescripcionFactura:  description,
		DetallesFactura:     lineDetails,
		ImporteTotalFactura: newImporteTotal(inv),
		RetencionSoportada:  newRetencionSoportada(inv),
		Claves:              &Claves{IDClave: newClaves(inv)},
	}, nil
}

func newDescription(notes []*org.Note) (string, error) {
	for _, note := range notes {
		if note.Key == org.NoteKeyGeneral {
			return note.Text, nil
		}
	}
	return "", validationErr(`notes: missing note with key '%s'`, org.NoteKeyGeneral)
}

func newImporteTotal(inv *bill.Invoice) string {
	totalWithDiscounts := inv.Totals.Total

	totalTaxes := num.MakeAmount(0, 2)
	if inv.Totals.Taxes != nil {
		for _, category := range inv.Totals.Taxes.Categories {
			if !category.Retained {
				totalTaxes = totalTaxes.Add(category.Amount)
			}
		}
	}

	return totalWithDiscounts.Add(totalTaxes).String()
}

func newRetencionSoportada(inv *bill.Invoice) string {
	totalRetention := num.MakeAmount(0, 2)
	if inv.Totals.Taxes != nil {
		for _, category := range inv.Totals.Taxes.Categories {
			if category.Retained {
				totalRetention = totalRetention.Add(category.Amount)
			}
		}
	}

	return totalRetention.String()
}

// newClaves returns the distinct ClaveRegimen codes from each VAT rate's
// es-tbai-regime extension.
func newClaves(inv *bill.Invoice) []IDClave {
	claves := []IDClave{}

	if inv.Totals != nil && inv.Totals.Taxes != nil {
		if cat := inv.Totals.Taxes.Category(tax.CategoryVAT); cat != nil {
			for _, rate := range cat.Rates {
				code := rate.Ext.Get(tbai.ExtKeyRegime).String()
				if code == "" || hasClave(claves, code) {
					continue
				}
				claves = append(claves, IDClave{ClaveRegimenIvaOpTrascendencia: code})
			}
		}
	}
	return claves
}

func hasClave(claves []IDClave, code string) bool {
	for _, c := range claves {
		if c.ClaveRegimenIvaOpTrascendencia == code {
			return true
		}
	}
	return false
}

func newFacturaRectificativa(inv *bill.Invoice) *FacturaRectificativa {
	if len(inv.Preceding) == 0 {
		return nil
	}

	p := inv.Preceding[0]

	return &FacturaRectificativa{
		Codigo: p.Ext.Get(tbai.ExtKeyCorrection).String(),
		Tipo:   CorrectiveTypeDifferences, // Only differences are supported for now
	}
}

func newFacturasRectificadasSustituidas(inv *bill.Invoice) *FacturasRectificadasSustituidas {
	if inv.Preceding == nil {
		return nil
	}

	p := inv.Preceding[0]

	return &FacturasRectificadasSustituidas{
		IDFacturaRectificadaSustituida: []*IDFacturaRectificadaSustituida{
			{
				SerieFactura:           p.Series.String(),
				NumFactura:             p.Code.String(),
				FechaExpedicionFactura: formatDate(p.IssueDate),
			},
		},
	}
}
