package gateways

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/invopop/gobl.es.ticketbai/convert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const cancelRejectedResponse = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<ns2:TicketBaiResponse xmlns:ns2="urn:ticketbai:anulacion">
	<Salida>
		<FechaRecepcion>05-08-2026 09:20:36</FechaRecepcion>
		<Estado>01</Estado>
		<Descripcion>Rechazado</Descripcion>
		<ResultadosValidacion>
			<Codigo>%s</Codigo>
			<Descripcion>%s</Descripcion>
		</ResultadosValidacion>
	</Salida>
</ns2:TicketBaiResponse>`

func TestAsCancelDuplicate(t *testing.T) {
	t.Run("already cancelled becomes a duplicate", func(t *testing.T) {
		err := asCancelDuplicate(
			ErrValidation.
				withCode("019").
				withMessage("El fichero de alta ya ha sido anulado previamente"),
		)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrDuplicate)
		assert.NotErrorIs(t, err, ErrValidation)

		var e *Error
		require.ErrorAs(t, err, &e)
		assert.Equal(t, "019", e.Code())
		assert.Equal(t, "El fichero de alta ya ha sido anulado previamente", e.Message())
	})

	t.Run("other validation errors are untouched", func(t *testing.T) {
		err := asCancelDuplicate(ErrValidation.withCode("005").withMessage("nope"))
		assert.ErrorIs(t, err, ErrValidation)
		assert.NotErrorIs(t, err, ErrDuplicate)
	})

	t.Run("nil stays nil", func(t *testing.T) {
		assert.NoError(t, asCancelDuplicate(nil))
	})

	t.Run("unknown errors are untouched", func(t *testing.T) {
		cause := errors.New("boom")
		assert.Equal(t, cause, asCancelDuplicate(cause))
	})
}

// TestRegionalCancelDuplicate ensures the regional gateways that share the
// common TicketBAI response format identify an already cancelled invoice.
func TestRegionalCancelDuplicate(t *testing.T) {
	conns := map[string]func(*tls.Config, string) Connection{
		"araba": func(tlsConf *tls.Config, url string) Connection {
			c := newAraba(EnvironmentSandbox, tlsConf)
			c.client.SetBaseURL(url)
			return c
		},
		"gipuzkoa": func(tlsConf *tls.Config, url string) Connection {
			c := newGipuzkoa(EnvironmentSandbox, tlsConf)
			c.client.SetBaseURL(url)
			return c
		},
	}

	responses := []struct {
		code    string
		message string
		target  error
	}{
		{"019", "El fichero de alta ya ha sido anulado previamente", ErrDuplicate},
		{"005", "El fichero de alta no existe", ErrValidation},
	}

	for name, newConn := range conns {
		for _, row := range responses {
			t.Run(name+"/"+row.code, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/xml")
					_, _ = fmt.Fprintf(w, cancelRejectedResponse, row.code, row.message)
				}))
				defer srv.Close()

				conn := newConn(new(tls.Config), srv.URL)
				err := conn.Cancel(context.Background(), nil, new(convert.AnulaTicketBAI))
				require.Error(t, err)
				assert.ErrorIs(t, err, row.target)

				var e *Error
				require.ErrorAs(t, err, &e)
				assert.Equal(t, row.code, e.Code())
				assert.Equal(t, row.message, e.Message())
			})
		}
	}
}

const cancelRejectedWithoutResultsResponse = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<ns2:TicketBaiResponse xmlns:ns2="urn:ticketbai:anulacion">
	<Salida>
		<FechaRecepcion>05-08-2026 09:20:36</FechaRecepcion>
		<Estado>01</Estado>
		<Descripcion>Rechazado</Descripcion>
	</Salida>
</ns2:TicketBaiResponse>`

// TestRegionalErrors ensures the regional gateways that share the common
// TicketBAI response format put the codes and messages from the gateway
// in the right place, whatever the HTTP status they arrive with.
func TestRegionalErrors(t *testing.T) {
	conns := map[string]func(*tls.Config, string) Connection{
		"araba": func(tlsConf *tls.Config, url string) Connection {
			c := newAraba(EnvironmentSandbox, tlsConf)
			c.client.SetBaseURL(url)
			return c
		},
		"gipuzkoa": func(tlsConf *tls.Config, url string) Connection {
			c := newGipuzkoa(EnvironmentSandbox, tlsConf)
			c.client.SetBaseURL(url)
			return c
		},
	}

	tests := []struct {
		name    string
		status  int
		body    string
		target  error
		code    string
		message string
	}{
		{
			name:    "rejected without validation results",
			status:  http.StatusOK,
			body:    cancelRejectedWithoutResultsResponse,
			target:  ErrValidation,
			code:    "",
			message: "Rechazado",
		},
		{
			name:    "accepted status without acknowledgement",
			status:  http.StatusOK,
			body:    "<TicketBaiResponse/>",
			target:  ErrConnection,
			code:    "",
			message: msgMissingStatus,
		},
		{
			name:    "rejected alongside error status",
			status:  http.StatusBadRequest,
			body:    fmt.Sprintf(cancelRejectedResponse, "005", "El fichero de alta no existe"),
			target:  ErrValidation,
			code:    "005",
			message: "El fichero de alta no existe",
		},
		{
			name:    "client error status",
			status:  http.StatusForbidden,
			body:    "Forbidden\n",
			target:  ErrValidation,
			code:    "403",
			message: "Forbidden",
		},
		{
			name:    "server error status",
			status:  http.StatusServiceUnavailable,
			body:    "<html><body>Service Unavailable</body></html>",
			target:  ErrServer,
			code:    "503",
			message: "<html><body>Service Unavailable</body></html>",
		},
	}

	for name, newConn := range conns {
		for _, tt := range tests {
			t.Run(name+"/"+tt.name, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/xml")
					w.WriteHeader(tt.status)
					_, _ = fmt.Fprint(w, tt.body)
				}))
				defer srv.Close()

				conn := newConn(new(tls.Config), srv.URL)
				err := conn.Cancel(context.Background(), nil, new(convert.AnulaTicketBAI))
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.target)

				var e *Error
				require.ErrorAs(t, err, &e)
				assert.Equal(t, tt.code, e.Code())
				assert.Equal(t, tt.message, e.Message())
			})
		}
	}
}

// TestRegionalAmendment ensures the regional gateways send amendments to the
// right service and identify amendments that were already registered.
func TestRegionalAmendment(t *testing.T) {
	conns := map[string]struct {
		new        func(*tls.Config, string) Connection
		altaPath   string
		cancelPath string
	}{
		"araba": {
			new: func(tlsConf *tls.Config, url string) Connection {
				c := newAraba(EnvironmentSandbox, tlsConf)
				c.client.SetBaseURL(url)
				return c
			},
			altaPath:   "/TicketBAI/v1/facturas/subsanarmodificar",
			cancelPath: "/TicketBAI/v1/anulaciones/baja",
		},
		"gipuzkoa": {
			new: func(tlsConf *tls.Config, url string) Connection {
				c := newGipuzkoa(EnvironmentSandbox, tlsConf)
				c.client.SetBaseURL(url)
				return c
			},
			altaPath:   "/sarrerak/zuzendu-alta",
			cancelPath: "/sarrerak/zuzendu-baja",
		},
	}

	send := map[string]func(Connection) error{
		"alta": func(c Connection) error {
			return c.PostAmendment(context.Background(), nil, new(convert.Amendment))
		},
		"cancel": func(c Connection) error {
			return c.CancelAmendment(context.Background(), nil, new(convert.CancelAmendment))
		},
	}

	responses := map[string][]struct {
		code   string
		target error
	}{
		"alta":   {{"020", ErrDuplicate}, {"021", ErrValidation}, {"023", ErrValidation}},
		"cancel": {{"022", ErrDuplicate}, {"018", ErrValidation}},
	}

	for name, conn := range conns {
		for kind, rows := range responses {
			path := conn.altaPath
			if kind == "cancel" {
				path = conn.cancelPath
			}
			for _, row := range rows {
				t.Run(name+"/"+kind+"/"+row.code, func(t *testing.T) {
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						assert.Equal(t, path, r.URL.Path)
						assert.Equal(t, "application/xml; charset=UTF-8", r.Header.Get("Content-Type"))
						w.Header().Set("Content-Type", "application/xml")
						_, _ = fmt.Fprintf(w, cancelRejectedResponse, row.code, "rejected")
					}))
					defer srv.Close()

					err := send[kind](conn.new(new(tls.Config), srv.URL))
					require.Error(t, err)
					assert.ErrorIs(t, err, row.target)

					var e *Error
					require.ErrorAs(t, err, &e)
					assert.Equal(t, row.code, e.Code())
				})
			}
		}
	}
}

func TestEBizkaiaAmendment(t *testing.T) {
	c := newEbizkaia(EnvironmentSandbox, new(tls.Config))
	assert.ErrorIs(t, c.PostAmendment(context.Background(), nil, nil), ErrValidation)
	assert.ErrorIs(t, c.CancelAmendment(context.Background(), nil, nil), ErrValidation)
}
