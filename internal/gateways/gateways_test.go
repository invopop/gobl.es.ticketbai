package gateways

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/invopop/gobl.ticketbai/convert"
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
					fmt.Fprintf(w, cancelRejectedResponse, row.code, row.message)
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
