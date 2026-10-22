package convert

import (
	"encoding/xml"
)

// Zuzendu is the correction service offered by the Gipuzkoa and Araba regions
// to resend TicketBAI documents that were rejected, or to modify those that were
// received with errors. Zuzendu documents are not signed. Instead they include
// the fingerprint and the first 100 characters of the signature of the original
// document, which means the TicketBAI code and QR of the invoice do not change.
//
//  * Gipuzkoa: https://www.gipuzkoa.eus/documents/2456431/27508852/cast+07+anexo+I+OF+16-2022.pdf/563fdd24-ff5c-05f1-876e-5e837de6c42f
//  * Araba: https://web.araba.eus/es/hacienda/ticketbai/documentacion-tecnica

const (
	zuzenduVersion = "1.0"

	zuzenduAltaNamespace = "urn:ticketbai:zuzendu-alta"
	zuzenduBajaNamespace = "urn:ticketbai:zuzendu-baja"
)

// ZuzenduAction defines what is being requested when sending an alta document
// through the Zuzendu service.
type ZuzenduAction string

// ZuzenduAction constants
const (
	// ZuzenduCorrect ("SUBSANAR") resends an alta document that was rejected,
	// and so never registered by the tax agency.
	ZuzenduCorrect ZuzenduAction = "SUBSANAR"
	// ZuzenduModify ("MODIFICAR") replaces the data of an alta document, or of a
	// previous correction, that was received by the tax agency with errors.
	ZuzenduModify ZuzenduAction = "MODIFICAR"
)

// ZuzenduAlta contains the data needed to correct or modify a TicketBAI invoice.
type ZuzenduAlta struct {
	XMLName    xml.Name `xml:"T:SubsanacionModificacionTicketBAI"`
	TNamespace string   `xml:"xmlns:T,attr"`

	Cabecera                   *ZuzenduCabecera
	Sujetos                    *Sujetos
	Factura                    *Factura
	HuellaTBAI                 *HuellaTBAI
	SignatureValueFirmaFactura string
}

// ZuzenduAnula contains the data needed to correct a TicketBAI cancellation
// that was rejected.
type ZuzenduAnula struct {
	XMLName    xml.Name `xml:"T:SubsanacionAnulacionTicketBAI"`
	TNamespace string   `xml:"xmlns:T,attr"`

	Cabecera                     *ZuzenduCabecera
	IDFactura                    *IDFactura
	HuellaTBAI                   *HuellaTBAI
	SignatureValueFirmaAnulacion string
}

// ZuzenduCabecera defines the head of Zuzendu documents. The action is only
// used by alta documents.
type ZuzenduCabecera struct {
	IDVersion string
	Accion    ZuzenduAction `xml:",omitempty"`
}

// NewZuzenduAlta prepares a Zuzendu document to correct or modify the original
// signed TicketBAI document with the data from the updated document. The
// invoice's issue date and time, fingerprint and signature are always taken
// from the original document, as these identify the invoice in the tax agency.
func NewZuzenduAlta(action ZuzenduAction, orig, doc *TicketBAI) (*ZuzenduAlta, error) {
	if action != ZuzenduCorrect && action != ZuzenduModify {
		return nil, validationErr("invalid zuzendu action '%s'", action)
	}
	if orig.SignatureValue() == "" || orig.HuellaTBAI == nil {
		return nil, validationErr("original document must be signed")
	}

	oh := orig.Head()
	h := *doc.Head() // copy so the updated document is left untouched
	if h.SerieFactura != oh.SerieFactura || h.NumFactura != oh.NumFactura {
		return nil, validationErr("invoice code does not match the original document")
	}
	if doc.Sujetos.Emisor.NIF != orig.Sujetos.Emisor.NIF {
		return nil, validationErr("supplier does not match the original document")
	}
	h.FechaExpedicionFactura = oh.FechaExpedicionFactura
	h.HoraExpedicionFactura = oh.HoraExpedicionFactura

	f := *doc.Factura
	f.CabeceraFactura = &h

	return &ZuzenduAlta{
		TNamespace: zuzenduAltaNamespace,
		Cabecera: &ZuzenduCabecera{
			IDVersion: zuzenduVersion,
			Accion:    action,
		},
		Sujetos:                    doc.Sujetos,
		Factura:                    &f,
		HuellaTBAI:                 orig.HuellaTBAI,
		SignatureValueFirmaFactura: trunc(orig.SignatureValue(), 100),
	}, nil
}

// NewZuzenduAnula prepares a Zuzendu document to correct the original signed
// cancellation document that was rejected.
func NewZuzenduAnula(orig *AnulaTicketBAI) (*ZuzenduAnula, error) {
	if orig.SignatureValue() == "" || orig.HuellaTBAI == nil {
		return nil, validationErr("original document must be signed")
	}

	return &ZuzenduAnula{
		TNamespace: zuzenduBajaNamespace,
		Cabecera: &ZuzenduCabecera{
			IDVersion: zuzenduVersion,
		},
		IDFactura:                    orig.IDFactura,
		HuellaTBAI:                   orig.HuellaTBAI,
		SignatureValueFirmaAnulacion: trunc(orig.SignatureValue(), 100),
	}, nil
}

// Bytes returns the XML document bytes
func (doc *ZuzenduAlta) Bytes() ([]byte, error) {
	return toBytes(doc)
}

// BytesIndent returns the indented XML document bytes
func (doc *ZuzenduAlta) BytesIndent() ([]byte, error) {
	return toBytesIndent(doc)
}

// Bytes returns the XML document bytes
func (doc *ZuzenduAnula) Bytes() ([]byte, error) {
	return toBytes(doc)
}

// BytesIndent returns the indented XML document bytes
func (doc *ZuzenduAnula) BytesIndent() ([]byte, error) {
	return toBytesIndent(doc)
}
