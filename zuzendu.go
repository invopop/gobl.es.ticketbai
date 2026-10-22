package ticketbai

import (
	"context"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl.es.ticketbai/convert"
	"github.com/invopop/gobl/bill"
)

// Expose Zuzendu actions for external use.
const (
	ZuzenduCorrect = convert.ZuzenduCorrect // Subsanar a rejected document
	ZuzenduModify  = convert.ZuzenduModify  // Modificar a document received with errors
)

// GenerateZuzendu creates a new Zuzendu document to correct or modify the
// original signed TicketBAI document using the current contents of the GOBL
// Envelope. The original document is required as Zuzendu documents are not
// signed, and instead refer to the original's fingerprint and signature, so
// the envelope's existing stamps remain valid.
//
// Use ZuzenduCorrect when the original document was rejected, and
// ZuzenduModify when it was received with errors. The Zuzendu service is only
// available in the Gipuzkoa and Araba regions.
func (c *Client) GenerateZuzendu(env *gobl.Envelope, orig *convert.TicketBAI, action convert.ZuzenduAction) (*convert.ZuzenduAlta, error) {
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return nil, ErrValidation.withMessage("only invoices are supported")
	}
	if orig == nil {
		return nil, ErrValidation.withMessage("missing original document")
	}

	doc, err := c.convertInvoice(inv)
	if err != nil {
		return nil, err
	}

	zd, err := convert.NewZuzenduAlta(action, orig, doc)
	if err != nil {
		return nil, ErrValidation.withCause(err)
	}

	return zd, nil
}

// GenerateZuzenduCancel creates a new Zuzendu document to correct the original
// signed cancellation document that was rejected.
func (c *Client) GenerateZuzenduCancel(orig *convert.AnulaTicketBAI) (*convert.ZuzenduAnula, error) {
	if orig == nil {
		return nil, ErrValidation.withMessage("missing original document")
	}

	zd, err := convert.NewZuzenduAnula(orig)
	if err != nil {
		return nil, ErrValidation.withCause(err)
	}

	return zd, nil
}

// PostZuzendu will send the Zuzendu document to the TicketBAI gateway.
func (c *Client) PostZuzendu(ctx context.Context, env *gobl.Envelope, d *convert.ZuzenduAlta) error {
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return ErrValidation.withMessage("only invoices are supported")
	}
	if err := c.gw.PostZuzendu(ctx, inv, d); err != nil {
		return newErrorFrom(err)
	}
	return nil
}

// CancelZuzendu will send the Zuzendu cancel document to the TicketBAI gateway.
func (c *Client) CancelZuzendu(ctx context.Context, env *gobl.Envelope, d *convert.ZuzenduAnula) error {
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return ErrValidation.withMessage("only invoices are supported")
	}
	if err := c.gw.CancelZuzendu(ctx, inv, d); err != nil {
		return newErrorFrom(err)
	}
	return nil
}
