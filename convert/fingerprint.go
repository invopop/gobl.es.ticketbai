package convert

// HuellaTBAI contains info about the Tickebai fingerprint
type HuellaTBAI struct {
	EncadenamientoFacturaAnterior *EncadenamientoFacturaAnterior `xml:",omitempty"`
	Software                      *Software
}

// EncadenamientoFacturaAnterior has the info of the previous invoice generated
type EncadenamientoFacturaAnterior struct {
	SerieFacturaAnterior               string `xml:",omitempty"`
	NumFacturaAnterior                 string
	FechaExpedicionFacturaAnterior     string
	SignatureValueFirmaFacturaAnterior string
}

// ChainData contains the fields of this invoice that will be
// required for fingerprinting the next invoice. JSON tags are
// provided to help with serialization.
type ChainData struct {
	Series    string `json:"series"`
	Code      string `json:"code"`
	IssueDate string `json:"issue_date"`
	Signature string `json:"signature"` // first 100 characters
}

// Software used to generate the Ticketbai invoice
type Software struct {
	License string `xml:"LicenciaTBAI"`
	NIF     string `xml:"EntidadDesarrolladora>NIF"`
	Name    string `xml:"Nombre"`
	Version string `xml:"Version"`
}

func newHuellaTBAI(soft *Software, data *ChainData) *HuellaTBAI {
	huella := &HuellaTBAI{
		EncadenamientoFacturaAnterior: nil,
		Software:                      soft,
	}

	if data != nil {
		huella.EncadenamientoFacturaAnterior = &EncadenamientoFacturaAnterior{
			SerieFacturaAnterior:               data.Series,
			NumFacturaAnterior:                 data.Code,
			FechaExpedicionFacturaAnterior:     data.IssueDate,
			SignatureValueFirmaFacturaAnterior: signaturePrefix(data.Signature),
		}
	}
	return huella
}

// signaturePrefixLength is the number of characters of a signature that
// TicketBAI documents use to refer to another document.
const signaturePrefixLength = 100

// signaturePrefix returns the start of the signature used to refer to it.
func signaturePrefix(s string) string {
	if len(s) <= signaturePrefixLength {
		return s
	}
	return s[:signaturePrefixLength]
}
