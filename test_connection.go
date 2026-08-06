package ticketbai

import (
	"context"

	"github.com/invopop/gobl.ticketbai/convert"
	"github.com/invopop/gobl.ticketbai/internal/gateways"
	"github.com/invopop/gobl/bill"
)

// TestConnection is a mock gateway connection for testing purposes
type TestConnection struct {
	postCalled   bool
	cancelCalled bool

	// PostError and CancelError, when set, are returned by the corresponding
	// method so that error handling can be exercised by tests.
	PostError   error
	CancelError error
}

var _ gateways.Connection = (*TestConnection)(nil)

// Post mocks the Post method of the Connection interface
func (tc *TestConnection) Post(_ context.Context, _ *bill.Invoice, _ *convert.TicketBAI) error {
	tc.postCalled = true
	return tc.PostError
}

// Cancel mocks the Cancel method of the Connection interface
func (tc *TestConnection) Cancel(_ context.Context, _ *bill.Invoice, _ *convert.AnulaTicketBAI) error {
	tc.cancelCalled = true
	return tc.CancelError
}
