package ticketbai

import (
	"context"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl.es.ticketbai/convert"
	"github.com/invopop/gobl/bill"
)

// Expose amendment actions for external use.
const (
	AmendRejected = convert.AmendRejected // Subsanar a rejected document
	AmendReceived = convert.AmendReceived // Modificar a document received with errors
)

// GenerateAmendment creates a new amendment document to resend or modify the
// original signed TicketBAI document using the current contents of the GOBL
// Envelope. The original document is required as amendments are not signed,
// and instead refer to the original's fingerprint and signature, so the
// envelope's existing stamps remain valid.
//
// Use AmendRejected when the original document was rejected, and AmendReceived
// when it was received with errors. Amendments are sent through the Zuzendu
// service, which is only available in the Gipuzkoa and Araba regions.
func (c *Client) GenerateAmendment(env *gobl.Envelope, orig *convert.TicketBAI, action convert.AmendmentAction) (*convert.Amendment, error) {
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

	ad, err := convert.NewAmendment(action, orig, doc)
	if err != nil {
		return nil, ErrValidation.withCause(err)
	}

	return ad, nil
}

// GenerateCancelAmendment creates a new amendment document to resend the original
// signed cancellation document that was rejected.
func (c *Client) GenerateCancelAmendment(orig *convert.AnulaTicketBAI) (*convert.CancelAmendment, error) {
	if orig == nil {
		return nil, ErrValidation.withMessage("missing original document")
	}

	ad, err := convert.NewCancelAmendment(orig)
	if err != nil {
		return nil, ErrValidation.withCause(err)
	}

	return ad, nil
}

// PostAmendment will send the amendment document to the TicketBAI gateway.
func (c *Client) PostAmendment(ctx context.Context, env *gobl.Envelope, d *convert.Amendment) error {
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return ErrValidation.withMessage("only invoices are supported")
	}
	if err := c.gw.PostAmendment(ctx, inv, d); err != nil {
		return newErrorFrom(err)
	}
	return nil
}

// CancelAmendment will send the cancel amendment document to the TicketBAI gateway.
func (c *Client) CancelAmendment(ctx context.Context, env *gobl.Envelope, d *convert.CancelAmendment) error {
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return ErrValidation.withMessage("only invoices are supported")
	}
	if err := c.gw.CancelAmendment(ctx, inv, d); err != nil {
		return newErrorFrom(err)
	}
	return nil
}
