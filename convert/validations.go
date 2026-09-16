package convert

import (
	"fmt"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/l10n"
)

// ValidationError is a simple wrapper around validation errors
type ValidationError struct {
	text string
}

// Error implements the error interface for ValidationError.
func (e *ValidationError) Error() string {
	return e.text
}

func validationErr(text string, args ...any) error {
	return &ValidationError{
		text: fmt.Sprintf(text, args...),
	}
}

var validSupplierLocalities = []l10n.Code{
	ZoneBI, // Vizcaya
	ZoneSS, // Guizpuzcoa
	ZoneVI, // Álava
}

func validate(inv *bill.Invoice, zone l10n.Code) error {
	if inv.Type == bill.InvoiceTypeCorrective {
		return validationErr("corrective invoices not supported, use credit or debit notes")
	}

	if inv.Supplier == nil || inv.Supplier.TaxID == nil {
		return nil // ignore
	}

	if zone == l10n.CodeEmpty {
		return validationErr("zone is required")
	}

	if !zone.In(validSupplierLocalities...) {
		return validationErr("zone not supported by TicketBAI")
	}

	if zone.In(ZoneSS, ZoneVI) {
		if len(inv.Lines) > 1000 {
			return validationErr("line count over limit (1000) for tax locality")
		}
		if inv.Customer != nil && len(inv.Customer.Addresses) == 0 {
			return validationErr("customer address required")
		}
	}

	// Line charges are reported by folding them into the line's unit price, as
	// TicketBAI has no field of its own for them. Charges on the invoice as a
	// whole belong to no line and so cannot be represented at all: reporting
	// them would leave the detail lines unable to add up to the invoice total.
	if len(inv.Charges) > 0 {
		return validationErr("invoice charges are not supported")
	}

	return nil
}
