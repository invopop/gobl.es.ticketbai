// Package convert contains the TicketBAI document structures and methods to generate it.
package convert

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/xmldsig"
)

// location is a shared location object set during init.
var location *time.Location

// TicketBAI zones
const (
	ZoneBI l10n.Code = "BI" // Vizcaya
	ZoneSS l10n.Code = "SS" // Guipúzcoa
	ZoneVI l10n.Code = "VI" // Álava
)

func init() {
	var err error
	location, err = time.LoadLocation("Europe/Madrid")
	if err != nil {
		panic(err)
	}
}

const (
	ticketBAIVersion = "1.2"

	ticketBAIEmisionNamespace   = "urn:ticketbai:emision"   // nolint:misspell
	ticketBAIAnulacionNamespace = "urn:ticketbai:anulacion" // nolint:misspell
)

// IssuerRole defines the role of the issuer in the invoice.
type IssuerRole string

// IssuerRole constants
const (
	IssuerRoleSupplier   IssuerRole = "N"
	IssuerRoleCustomer   IssuerRole = "D"
	IssuerRoleThirdParty IssuerRole = "T"
)

// CorrectiveType constants
const (
	CorrectiveTypeSubstitution = "S"
	CorrectiveTypeDifferences  = "I"
)

// TicketBAI contains the data needed to create a TicketBAI invoice
type TicketBAI struct {
	XMLName    xml.Name `xml:"T:TicketBai"`
	TNamespace string   `xml:"xmlns:T,attr"`

	Cabecera   *Cabecera          // Header
	Sujetos    *Sujetos           // Subjects
	Factura    *Factura           // Invoice
	HuellaTBAI *HuellaTBAI        // Fingerprint
	Signature  *xmldsig.Signature `xml:"ds:Signature,omitempty"` // XML Signature

	ts time.Time
}

// Cabecera defines the document head with TBAI version ID.
type Cabecera struct {
	IDVersionTBAI string
}

// NewTicketBAI takes the GOBL Invoice and converts into a TicketBAI document
// ready to send to a regional API.
func NewTicketBAI(inv *bill.Invoice, ts time.Time, role IssuerRole, zone l10n.Code) (*TicketBAI, error) {
	err := validate(inv, zone)
	if err != nil {
		return nil, err
	}

	// GOBL credit note's amounts represent the amounts to be credited to the customer,
	// and they are provided as positive numbers. In TicketBAI, however, credit notes
	// become "facturas rectificativas por diferencias" and, when a correction is for a
	// credit operation, the amounts must be negative to cancel out the ones in the
	// original invoice. The amounts are negated as they are output instead of inverting
	// and recalculating the invoice, so that the source document is left untouched and
	// the original totals are used as provided, including with the bypass tag.
	negate := inv.Type == bill.InvoiceTypeCreditNote

	doc := &TicketBAI{
		TNamespace: ticketBAIEmisionNamespace,
		Cabecera: &Cabecera{
			IDVersionTBAI: ticketBAIVersion,
		},
		Sujetos: &Sujetos{
			Emisor:                          newEmisor(inv.Supplier),
			EmitidaPorTercerosODestinatario: string(role),
		},
		Factura: &Factura{
			CabeceraFactura: newCabeceraFactura(inv),
			TipoDesglose:    newTipoDesglose(inv, negate),
		},
	}

	doc.SetIssueTimestamp(ts)

	// Add customers
	if inv.Customer != nil {
		// If the customer is still nil, implies that they didn't have enough
		// fiscal information to include in the output.
		if dest := newDestinatario(inv.Customer); dest != nil {
			doc.Sujetos.Destinatarios = &Destinatarios{
				IDDestinatario: []*IDDestinatario{dest},
			}
		}
	}

	// Complete invoice data
	doc.Factura.DatosFactura, err = newDatosFactura(inv, negate)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// SetIssueTimestamp sets the issue date and time for the document
func (doc *TicketBAI) SetIssueTimestamp(ts time.Time) {
	doc.ts = ts

	doc.Factura.CabeceraFactura.FechaExpedicionFactura = formatDate(ts)
	doc.Factura.CabeceraFactura.HoraExpedicionFactura = formatTime(ts)
}

// IssueTimestamp returns the issue date and time for the document
func (doc *TicketBAI) IssueTimestamp() time.Time {
	return doc.ts
}

// IssueYear returns the year of the issue date
func (doc *TicketBAI) IssueYear() string {
	if doc.Factura == nil ||
		doc.Factura.CabeceraFactura == nil ||
		doc.Factura.CabeceraFactura.FechaExpedicionFactura == "" {
		return ""
	}
	year := doc.Factura.CabeceraFactura.FechaExpedicionFactura
	year = year[len(year)-4:] // last four as in "01-02-2022" format
	return year
}

// Fingerprint tries to generate the "HuellaTBAI" using the
// previous invoice details (if available) as a reference.
func (doc *TicketBAI) Fingerprint(soft *Software, data *ChainData) error {
	doc.HuellaTBAI = newHuellaTBAI(soft, data)
	return nil
}

// Sign signs the document with the given certificate and role
func (doc *TicketBAI) Sign(docID string, cert *xmldsig.Certificate, role IssuerRole, zone l10n.Code, opts ...xmldsig.Option) error {
	s, err := newSignature(doc, docID, zone, role, cert, opts...)
	if err != nil {
		return err
	}

	doc.Signature = s

	return nil
}

// ChainData generates the data to be used to link to this one
// in the next entry.
func (doc *TicketBAI) ChainData() *ChainData {
	h := doc.Head()
	sig := trunc(doc.SignatureValue(), 100)
	return &ChainData{
		Series:    h.SerieFactura,
		Code:      h.NumFactura,
		IssueDate: h.FechaExpedicionFactura,
		Signature: sig,
	}
}

// Head returns the CabeceraFactura.
func (doc *TicketBAI) Head() *CabeceraFactura {
	return doc.Factura.CabeceraFactura
}

// QRCodes generates the QR codes for this invoice, but requires the Fingerprint to have been
// generated first.
func (doc *TicketBAI) QRCodes(zone l10n.Code) *Codes {
	if doc.HuellaTBAI == nil {
		return nil
	}
	return doc.generateCodes(zone)
}

// Bytes returns the XML document bytes
func (doc *TicketBAI) Bytes() ([]byte, error) {
	return toBytes(doc)
}

// BytesIndent returns the indented XML document bytes
func (doc *TicketBAI) BytesIndent() ([]byte, error) {
	return toBytesIndent(doc)
}

func toBytes(doc any) ([]byte, error) {
	buf, err := buffer(doc, xml.Header, false)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func toBytesIndent(doc any) ([]byte, error) {
	buf, err := buffer(doc, xml.Header, true)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func toBytesCanonical(doc any) ([]byte, error) {
	buf, err := buffer(doc, "", false)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func buffer(doc any, base string, indent bool) (*bytes.Buffer, error) {
	buf := bytes.NewBufferString(base)

	enc := xml.NewEncoder(buf)
	if indent {
		enc.Indent("", "  ")
	}

	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("encoding document: %w", err)
	}

	return buf, nil
}

type timeLocationable interface {
	In(*time.Location) time.Time
}

func formatDate(ts timeLocationable) string {
	return ts.In(location).Format("02-01-2006")
}

func formatTime(ts timeLocationable) string {
	return ts.In(location).Format("15:04:05")
}
