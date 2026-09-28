package gateways

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/invopop/gobl.ticketbai/internal/gateways/ebizkaia"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEBizkaiaSendRequestErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		headers map[string]string
		body    string
		target  error
		code    string
		message string
	}{
		{
			name:   "validation error in headers",
			status: http.StatusOK,
			headers: map[string]string{
				eBizkaiaN3ResponseHeader: eBizkaiaN3ResponseInvalid,
				eBizkaiaN3RespCodeHeader: "B4_2000013",
				eBizkaiaN3MessageHeader:  "NIF-IVA tiene un formato erróneo",
			},
			target:  ErrValidation,
			code:    "B4_2000013",
			message: "NIF-IVA tiene un formato erróneo",
		},
		{
			name:   "technical error in headers",
			status: http.StatusOK,
			headers: map[string]string{
				eBizkaiaN3ResponseHeader: eBizkaiaN3ResponseInvalid,
				eBizkaiaN3RespCodeHeader: eBizkaiaN3RespCodeTechnical,
				eBizkaiaN3MessageHeader:  "Error técnico",
			},
			target:  ErrServer,
			code:    eBizkaiaN3RespCodeTechnical,
			message: "Error técnico",
		},
		{
			name:   "headers alongside error status",
			status: http.StatusInternalServerError,
			headers: map[string]string{
				eBizkaiaN3ResponseHeader: eBizkaiaN3ResponseInvalid,
				eBizkaiaN3RespCodeHeader: eBizkaiaN3RespCodeOther,
				eBizkaiaN3MessageHeader:  "Otros, consulte el mensaje recibido",
			},
			target:  ErrServer,
			code:    eBizkaiaN3RespCodeOther,
			message: "Otros, consulte el mensaje recibido",
		},
		{
			name:    "client error status",
			status:  http.StatusBadRequest,
			body:    "Bad Request",
			target:  ErrValidation,
			code:    "400",
			message: "Bad Request",
		},
		{
			name:    "server error status",
			status:  http.StatusBadGateway,
			body:    "Bad Gateway",
			target:  ErrServer,
			code:    "502",
			message: "Bad Gateway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				for k, v := range tt.headers {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tt.status)
				_, _ = fmt.Fprint(w, tt.body)
			}))
			defer srv.Close()

			c := newEbizkaia(EnvironmentSandbox, new(tls.Config))
			c.client.SetBaseURL(srv.URL)

			req := &ebizkaia.Request{Header: []byte("{}"), Payload: []byte("payload")}
			err := c.sendRequest(context.Background(), req, eBizkaiaExecutePath, nil)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.target)

			var e *Error
			require.ErrorAs(t, err, &e)
			assert.Equal(t, tt.code, e.Code())
			assert.Equal(t, tt.message, e.Message())
		})
	}

	t.Run("accepted", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set(eBizkaiaN3ResponseHeader, "Correcto")
		}))
		defer srv.Close()

		c := newEbizkaia(EnvironmentSandbox, new(tls.Config))
		c.client.SetBaseURL(srv.URL)

		req := &ebizkaia.Request{Header: []byte("{}"), Payload: []byte("payload")}
		assert.NoError(t, c.sendRequest(context.Background(), req, eBizkaiaExecutePath, nil))
	})
}
