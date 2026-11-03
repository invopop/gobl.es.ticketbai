// Package gateways contains the different interfaces to send the TicketBAI
// documents to.
package gateways

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/invopop/gobl.es.ticketbai/ca"
	"github.com/invopop/gobl.es.ticketbai/convert"
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
	// ErrConnection is used when the gateway could not be reached or gave a
	// response we're unable to understand.
	ErrConnection = newError("connection")
	// ErrValidation implies there is something wrong with the contents of the
	// request that needs to be fixed before sending it again.
	ErrValidation = newError("validation")
	// ErrDuplicate is used when the gateway has already received the request.
	ErrDuplicate = newError("duplicate")
	// ErrServer indicates the gateway had an internal problem handling the
	// request, which may succeed if attempted again later.
	ErrServer = newError("server")
)

// msgMissingStatus is reported when the gateway responds successfully but
// without the status that confirms the document was received.
const msgMissingStatus = "response does not include a status"

// Validation codes from the common TicketBAI specification, shared by the
// Araba and Gipuzkoa gateways.
const (
	// codeAlreadyCancelled is returned when the invoice being cancelled has
	// already been cancelled previously: "El fichero de alta ya ha sido
	// anulado previamente".
	codeAlreadyCancelled = "019"

	// codeAmendmentRegistered is returned by the Zuzendu service when the
	// alta document being corrected has already been registered: "Fichero de
	// alta TicketBAI a subsanar ha sido recibido previamente en el sistema".
	codeAmendmentRegistered = "020"

	// codeCancelAmendmentRegistered is returned by the Zuzendu service when the
	// cancellation being corrected has already been registered: "Fichero de
	// anulación TicketBAI a subsanar no debe haber sido recibido previamente
	// en el sistema".
	codeCancelAmendmentRegistered = "022"
)

// asCancelDuplicate converts the validation responses that imply the document
// has already been cancelled into an ErrDuplicate, so that repeating a cancel
// request can be handled as a no-op upstream.
func asCancelDuplicate(err error) error {
	return asDuplicate(err, codeAlreadyCancelled)
}

// asDuplicate converts validation responses with any of the given codes into
// an ErrDuplicate.
func asDuplicate(err error, codes ...string) error {
	var e *Error
	if errors.As(err, &e) && slices.Contains(codes, e.Code()) {
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

// statusError converts an unexpected HTTP status into an error. The status is
// used as the code and the body as the message, as it usually describes what
// went wrong in more detail than the status alone. Server errors are reported
// separately so the request can be attempted again later.
func statusError(res *resty.Response) *Error {
	e := ErrValidation
	if res.StatusCode() >= http.StatusInternalServerError {
		e = ErrServer
	}
	return e.
		withCode(strconv.Itoa(res.StatusCode())).
		withMessage(strings.TrimSpace(res.String()))
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
	// PostAmendment sends a correction or modification of an alta document to the
	// Zuzendu service.
	PostAmendment(ctx context.Context, inv *bill.Invoice, doc *convert.Amendment) error
	// CancelAmendment sends a correction of a cancellation document to the
	// Zuzendu service.
	CancelAmendment(ctx context.Context, inv *bill.Invoice, doc *convert.CancelAmendment) error
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
