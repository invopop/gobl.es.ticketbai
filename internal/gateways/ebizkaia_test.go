package gateways

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/invopop/gobl.es.ticketbai/convert"
	"github.com/invopop/gobl.es.ticketbai/internal/gateways/ebizkaia"
	"github.com/invopop/gobl.es.ticketbai/test"
	"github.com/invopop/gobl/cbc"
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
			name:    "throttled status",
			status:  http.StatusTooManyRequests,
			body:    "Too Many Requests",
			target:  ErrServer,
			code:    "429",
			message: "Too Many Requests",
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

func TestEBizkaiaPostModel(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		code    cbc.Code
		want    string
	}{
		{name: "company", fixture: "sample-invoice.json", want: "240"},
		{name: "entity under income attribution", fixture: "invoice-bi-pf-modelo140.json", code: "E12345674", want: "140"},
		{name: "civil partnership", fixture: "invoice-bi-pf-modelo140.json", code: "J12345674", want: "140"},
		{name: "community of owners", fixture: "invoice-bi-pf-modelo140.json", code: "H12345674", want: "140"},
		{name: "individual", fixture: "invoice-bi-pf-modelo140.json", want: "140"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var n3Data string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n3Data = r.Header.Get(eBizkaiaN3DataHeader)
				w.Header().Set(eBizkaiaN3ResponseHeader, "Correcto")
			}))
			defer srv.Close()

			c := newEbizkaia(EnvironmentSandbox, new(tls.Config))
			c.client.SetBaseURL(srv.URL)

			inv := test.LoadInvoice(tt.fixture)
			if tt.code != "" {
				inv.Supplier.TaxID.Code = tt.code
			}
			doc, err := convert.NewTicketBAI(inv, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), convert.IssuerRoleSupplier, convert.ZoneBI)
			require.NoError(t, err)
			_ = c.Post(context.Background(), inv, doc)
			require.NotEmpty(t, n3Data, "Post didn't send a request")

			var n3 struct {
				Drs struct {
					Mode string `json:"mode"`
				} `json:"drs"`
			}
			require.NoError(t, json.Unmarshal([]byte(n3Data), &n3))
			assert.Equal(t, tt.want, n3.Drs.Mode)
		})
	}
}
