package ticketbai

import (
	"context"

	"github.com/invopop/gobl.es.ticketbai/convert"
	"github.com/invopop/gobl.es.ticketbai/internal/gateways"
	"github.com/invopop/gobl/bill"
)

// TestConnection is a mock gateway connection for testing purposes
type TestConnection struct {
	postCalled          bool
	cancelCalled        bool
	postZuzenduCalled   bool
	cancelZuzenduCalled bool

	// PostError, CancelError, PostZuzenduError and CancelZuzenduError, when
	// set, are returned by the corresponding method so that error handling can
	// be exercised by tests.
	PostError          error
	CancelError        error
	PostZuzenduError   error
	CancelZuzenduError error
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

// PostZuzendu mocks the PostZuzendu method of the Connection interface
func (tc *TestConnection) PostZuzendu(_ context.Context, _ *bill.Invoice, _ *convert.ZuzenduAlta) error {
	tc.postZuzenduCalled = true
	return tc.PostZuzenduError
}

// CancelZuzendu mocks the CancelZuzendu method of the Connection interface
func (tc *TestConnection) CancelZuzendu(_ context.Context, _ *bill.Invoice, _ *convert.ZuzenduAnula) error {
	tc.cancelZuzenduCalled = true
	return tc.CancelZuzenduError
}
