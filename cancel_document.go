package ticketbai

import (
	"fmt"
	"strings"
	"time"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl.es.ticketbai/convert"
	"github.com/invopop/gobl/addons/es/tbai"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/xmldsig"
)

// GenerateCancel creates a new AnulaTicketBAI document from the provided
// GOBL Envelope.
func (c *Client) GenerateCancel(env *gobl.Envelope) (*convert.AnulaTicketBAI, error) {
	// Extract the Invoice
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return nil, ErrValidation.withMessage("only invoices are supported")
	}
	if inv.Supplier.TaxID.Country != l10n.ES.Tax() {
		return nil, ErrValidation.withMessage("only spanish invoices are supported")
	}
	zone := zoneFor(inv)
	if zone == "" {
		return nil, ErrValidation.withMessage("invalid zone")
	}

	// Extract the time when the invoice was posted to TicketBAI gateway
	ts, err := extractPostTime(env)
	if err != nil {
		return nil, err
	}

	// Create the document
	cd, err := convert.NewAnulaTicketBAI(inv, ts)
	if err != nil {
		return nil, err
	}

	return cd, nil
}

// FingerprintCancel generates a finger print for the TicketBAI document using the
// data provided from the previous invoice data.
func (c *Client) FingerprintCancel(cd *convert.AnulaTicketBAI) error {
	conf := c.buildSoftware()
	return cd.Fingerprint(conf)
}

// SignCancel is used to generate the XML DSig components of the final XML document.
func (c *Client) SignCancel(cd *convert.AnulaTicketBAI, env *gobl.Envelope) error {
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return ErrValidation.withMessage("only invoices are supported")
	}
	zone := zoneFor(inv)
	if zone == "" {
		return ErrValidation.withMessage("invalid zone: '%s'", zone)
	}
	dID := env.Head.UUID.String()
	if err := cd.Sign(dID, c.cert, c.issuerRole, zone, xmldsig.WithCurrentTime(c.CurrentTime)); err != nil {
		return fmt.Errorf("signing: %w", err)
	}

	return nil
}

func extractPostTime(env *gobl.Envelope) (time.Time, error) {
	for _, stamp := range env.Head.Stamps {
		if stamp.Provider == tbai.StampCode {
			parts := strings.Split(stamp.Value, "-")
			ts, err := time.Parse("020106", parts[2])
			if err != nil {
				return time.Time{}, fmt.Errorf("parsing previous invoice date: %w", err)
			}

			return ts, nil
		}
	}

	return time.Time{}, fmt.Errorf("missing previous %s stamp in envelope", tbai.StampCode)
}
