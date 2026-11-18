package gateways

import (
	"bytes"
	"compress/gzip"
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

func TestEBizkaiaPostRejections(t *testing.T) {
	const record = `<LROEPJ240FacturasEmitidasConSGAltaRespuesta><Registros><Registro><SituacionRegistro>` +
		`<CodigoErrorRegistro>B4_2000003</CodigoErrorRegistro>` +
		`<DescripcionErrorRegistroES>El registro no puede existir en el sistema</DescripcionErrorRegistroES>` +
		`</SituacionRegistro></Registro></Registros></LROEPJ240FacturasEmitidasConSGAltaRespuesta>`
	rejected := map[string]string{
		"Content-Type":           "application/xml; charset=UTF-8",
		eBizkaiaN3ResponseHeader: eBizkaiaN3ResponseInvalid,
		eBizkaiaN3RespCodeHeader: "B4_1000006",
		eBizkaiaN3MessageHeader:  "El Modelo no corresponde al tipo de persona.",
	}

	tests := []struct {
		name    string
		status  int
		headers map[string]string
		body    string
		gzip    bool
		chunked bool
		target  error
		code    string
	}{
		{
			name:    "rejection with an empty body",
			status:  http.StatusOK,
			headers: rejected,
			gzip:    true,
			target:  ErrValidation,
			code:    "B4_1000006",
		},
		{
			name:    "rejection with an empty chunked body",
			status:  http.StatusOK,
			headers: rejected,
			gzip:    true,
			chunked: true,
			target:  ErrValidation,
			code:    "B4_1000006",
		},
		{
			name:    "rejection with an unreadable body",
			status:  http.StatusOK,
			headers: rejected,
			body:    record[:len(record)-len("</Registros></LROEPJ240FacturasEmitidasConSGAltaRespuesta>")],
			target:  ErrValidation,
			code:    "B4_1000006",
		},
		{
			name:   "duplicate record in the body",
			status: http.StatusOK,
			headers: map[string]string{
				"Content-Type":           "application/xml; charset=UTF-8",
				eBizkaiaN3ResponseHeader: eBizkaiaN3ResponseInvalid,
				eBizkaiaN3RespCodeHeader: "B4_1000002",
			},
			body:   record,
			gzip:   true,
			target: ErrDuplicate,
			code:   "B4_2000003",
		},
		{
			name:    "client error status with a record body",
			status:  http.StatusBadRequest,
			headers: map[string]string{"Content-Type": "application/xml; charset=UTF-8"},
			body:    record,
			target:  ErrValidation,
			code:    "400",
		},
	}

	inv := test.LoadInvoice("sample-invoice.json")
	doc, err := convert.NewTicketBAI(inv, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), convert.IssuerRoleSupplier, convert.ZoneBI)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var acceptEncoding string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				acceptEncoding = r.Header.Get("Accept-Encoding")
				for k, v := range tt.headers {
					w.Header().Set(k, v)
				}
				body := []byte(tt.body)
				if tt.gzip {
					w.Header().Set("Content-Encoding", "gzip")
					if len(body) > 0 {
						var buf bytes.Buffer
						zw := gzip.NewWriter(&buf)
						_, _ = zw.Write(body)
						_ = zw.Close()
						body = buf.Bytes()
					}
				}
				w.WriteHeader(tt.status)
				if tt.chunked {
					w.(http.Flusher).Flush()
				}
				_, _ = w.Write(body)
			}))
			defer srv.Close()

			c := newEbizkaia(EnvironmentSandbox, new(tls.Config))
			c.client.SetBaseURL(srv.URL)

			err := c.Post(context.Background(), inv, doc)
			assert.ErrorIs(t, err, tt.target)
			var e *Error
			require.ErrorAs(t, err, &e)
			assert.Equal(t, tt.code, e.Code())
			assert.Equal(t, "gzip", acceptEncoding)
		})
	}
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
