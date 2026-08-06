// Package gateways contains the different interfaces to send the TicketBAI
// documents to.
package gateways

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/invopop/gobl.ticketbai/ca"
	"github.com/invopop/gobl.ticketbai/convert"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/xmldsig"
)

// Environment defines the environment to use for connections
type Environment string

// Environment to use for connections
const (
	EnvironmentProduction Environment = "production"
	EnvironmentSandbox    Environment = "sandbox"
)

// Standard gateway error responses. Keys match the ones from main package.
var (
	ErrConnection = newError("connection")
	ErrValidation = newError("validation")
	ErrDuplicate  = newError("duplicate")
)

// Validation codes from the common TicketBAI specification, shared by the
// Araba and Gipuzkoa gateways.
const (
	// codeAlreadyCancelled is returned when the invoice being cancelled has
	// already been cancelled previously: "El fichero de alta ya ha sido
	// anulado previamente".
	codeAlreadyCancelled = "019"
)

// asCancelDuplicate converts the validation responses that imply the document
// has already been cancelled into an ErrDuplicate, so that repeating a cancel
// request can be handled as a no-op upstream.
func asCancelDuplicate(err error) error {
	var e *Error
	if errors.As(err, &e) && e.Code() == codeAlreadyCancelled {
		return ErrDuplicate.withCode(e.Code()).withMessage(e.Message())
	}
	return err
}

// Error allows for structured responses from the gateway to be able to
// response codes and messages.
type Error struct {
	key     string
	code    string
	message string
	cause   error
}

// Error produces a human readable error message.
func (e *Error) Error() string {
	out := []string{e.key}
	if e.code != "" {
		out = append(out, e.code)
	}
	if e.message != "" {
		out = append(out, e.message)
	}
	return strings.Join(out, ": ")
}

// Key returns the key for the error.
func (e *Error) Key() string {
	return e.key
}

// Message returns the human message for the error.
func (e *Error) Message() string {
	return e.message
}

// Code returns the code provided by the remote service.
func (e *Error) Code() string {
	return e.code
}

func newError(key string) *Error {
	return &Error{key: key}
}

// withCode duplicates and adds the code to the error.
func (e *Error) withCode(code string) *Error {
	e = e.clone()
	e.code = code
	return e
}

// withMessage duplicates and adds the message to the error.
func (e *Error) withMessage(msg string) *Error {
	e = e.clone()
	e.message = msg
	return e
}

func (e *Error) withCause(err error) *Error {
	e = e.clone()
	e.cause = err
	e.message = err.Error()
	return e
}

func (e *Error) clone() *Error {
	ne := new(Error)
	*ne = *e
	return ne
}

// Is checks to see if the target error is the same as the current one
// or forms part of the chain.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return errors.Is(e.cause, target)
	}
	return e.key == t.key
}

// Connection defines what is expected from a connection to a gateway.
type Connection interface {
	// Post sends the complete TicketBAI document to the remote end-point. We assume
	// the document has been fully prepared and signed.
	Post(ctx context.Context, inv *bill.Invoice, doc *convert.TicketBAI) error
	Cancel(ctx context.Context, inv *bill.Invoice, doc *convert.AnulaTicketBAI) error
}

// New instantiates a new connection for the given zone and environment.
func New(env Environment, zone l10n.Code, cert *xmldsig.Certificate) (Connection, error) {
	tlsConf, err := cert.TLSAuthConfig()
	if err != nil {
		return nil, fmt.Errorf("preparing TLS config: %w", err)
	}

	certs, err := rootCAPool()
	if err != nil {
		return nil, fmt.Errorf("preparing cert pool: %w", err)
	}

	tlsConf.RootCAs = certs
	tlsConf.Renegotiation = tls.RenegotiateOnceAsClient

	switch zone {
	case convert.ZoneBI:
		return newEbizkaia(env, tlsConf), nil
	case convert.ZoneSS:
		return newGipuzkoa(env, tlsConf), nil
	case convert.ZoneVI:
		return newAraba(env, tlsConf), nil
	}

	return nil, fmt.Errorf("zone %s not supported", zone)
}

func rootCAPool() (*x509.CertPool, error) {
	certs := x509.NewCertPool()
	files, err := ca.Content.ReadDir(".")
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		data, err := ca.Content.ReadFile(f.Name())
		if err != nil {
			return nil, err
		}
		certs.AppendCertsFromPEM(data)
	}
	return certs, nil
}

func debug() bool {
	return os.Getenv("DEBUG") == "true"
}
