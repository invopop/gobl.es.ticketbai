package convert

import (
	"encoding/xml"
)

// Amendments resend TicketBAI documents that were rejected, or modify those that
// were received with errors, without issuing a corrective invoice. They are sent
// through the "Zuzendu" service (Basque for "to correct") offered by the
// Gipuzkoa and Araba regions. Amendments are not signed. Instead they include
// the fingerprint and the first 100 characters of the signature of the original
// document, which means the TicketBAI code and QR of the invoice do not change.
//
//  * Gipuzkoa: https://www.gipuzkoa.eus/documents/2456431/27508852/cast+07+anexo+I+OF+16-2022.pdf/563fdd24-ff5c-05f1-876e-5e837de6c42f
//  * Araba: https://web.araba.eus/es/hacienda/ticketbai/documentacion-tecnica

const (
	amendmentVersion = "1.0"

	amendmentNamespace       = "urn:ticketbai:zuzendu-alta"
	cancelAmendmentNamespace = "urn:ticketbai:zuzendu-baja"
)

// AmendmentAction defines what is being requested when sending an alta document
// as an amendment.
type AmendmentAction string

// AmendmentAction constants
const (
	// AmendRejected ("SUBSANAR") resends an alta document that was rejected,
	// and so never registered by the tax agency.
	AmendRejected AmendmentAction = "SUBSANAR"
	// AmendReceived ("MODIFICAR") replaces the data of an alta document, or of a
	// previous correction, that was received by the tax agency with errors.
	AmendReceived AmendmentAction = "MODIFICAR"
)

// Amendment contains the data needed to correct or modify a TicketBAI invoice.
type Amendment struct {
	XMLName    xml.Name `xml:"T:SubsanacionModificacionTicketBAI"`
	TNamespace string   `xml:"xmlns:T,attr"`

	Cabecera                   *AmendmentCabecera
	Sujetos                    *Sujetos
	Factura                    *Factura
	HuellaTBAI                 *HuellaTBAI
	SignatureValueFirmaFactura string
}

// CancelAmendment contains the data needed to correct a TicketBAI cancellation
// that was rejected.
type CancelAmendment struct {
	XMLName    xml.Name `xml:"T:SubsanacionAnulacionTicketBAI"`
	TNamespace string   `xml:"xmlns:T,attr"`

	Cabecera                     *AmendmentCabecera
	IDFactura                    *IDFactura
	HuellaTBAI                   *HuellaTBAI
	SignatureValueFirmaAnulacion string
}

// AmendmentCabecera defines the head of amendment documents. The action is only
// used by alta documents.
type AmendmentCabecera struct {
	IDVersion string
	Accion    AmendmentAction `xml:",omitempty"`
}

// NewAmendment prepares an amendment to correct or modify the original
// signed TicketBAI document with the data from the updated document. The
// invoice's issue date and time, fingerprint and signature are always taken
// from the original document, as these identify the invoice in the tax agency.
func NewAmendment(action AmendmentAction, orig, doc *TicketBAI) (*Amendment, error) {
	if action != AmendRejected && action != AmendReceived {
		return nil, validationErr("invalid amendment action '%s'", action)
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

	return &Amendment{
		TNamespace: amendmentNamespace,
		Cabecera: &AmendmentCabecera{
			IDVersion: amendmentVersion,
			Accion:    action,
		},
		Sujetos:                    doc.Sujetos,
		Factura:                    &f,
		HuellaTBAI:                 orig.HuellaTBAI,
		SignatureValueFirmaFactura: trunc(orig.SignatureValue(), 100),
	}, nil
}

// NewCancelAmendment prepares an amendment to correct the original signed
// cancellation document that was rejected.
func NewCancelAmendment(orig *AnulaTicketBAI) (*CancelAmendment, error) {
	if orig.SignatureValue() == "" || orig.HuellaTBAI == nil {
		return nil, validationErr("original document must be signed")
	}

	return &CancelAmendment{
		TNamespace: cancelAmendmentNamespace,
		Cabecera: &AmendmentCabecera{
			IDVersion: amendmentVersion,
		},
		IDFactura:                    orig.IDFactura,
		HuellaTBAI:                   orig.HuellaTBAI,
		SignatureValueFirmaAnulacion: trunc(orig.SignatureValue(), 100),
	}, nil
}

// Bytes returns the XML document bytes
func (doc *Amendment) Bytes() ([]byte, error) {
	return toBytes(doc)
}

// BytesIndent returns the indented XML document bytes
func (doc *Amendment) BytesIndent() ([]byte, error) {
	return toBytesIndent(doc)
}

// Bytes returns the XML document bytes
func (doc *CancelAmendment) Bytes() ([]byte, error) {
	return toBytes(doc)
}

// BytesIndent returns the indented XML document bytes
func (doc *CancelAmendment) BytesIndent() ([]byte, error) {
	return toBytesIndent(doc)
}
