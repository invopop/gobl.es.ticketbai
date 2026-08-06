package gateways

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-resty/resty/v2"
	"github.com/invopop/gobl.ticketbai/convert"
	"github.com/invopop/gobl/bill"
)

//
// Gipuzkoa region
//
//  * Docs: https://www.gipuzkoa.eus/es/web/ogasuna/ticketbai/documentacion-y-normativa
//  * Testing: https://www.gipuzkoa.eus/documents/2456431/13761128/Guia+de+entono+de+pruebas+TicketBAI.pdf/d8b052ec-d40a-7f9c-0961-fcf7a631a948
//  * Production: https://www.gipuzkoa.eus/documents/2456431/27508852/cast+03+ANEXO+IV+web+26-01-2022.pdf/a991d1ae-2b1c-f039-36a6-ae9220cc7127

const (
	// Requests
	gipuzkoaProductionBaseURL = "https://tbai-z.egoitza.gipuzkoa.eus"
	gipuzkoaTestingBaseURL    = "https://tbai-z.prep.gipuzkoa.eus"

	gipuzkoaExecutePath = "/sarrerak/alta"
	gipuzkoaCancelPath  = "/sarrerak/baja"
)

const (
	gipuzkoaStatusReceived = "00" // looks good.
	gipuzkoaStatusRejected = "01" // there are errors that need fixing.
)

// GipuzkoaResponse defines the response fields from the Gipuzkoa region.
type GipuzkoaResponse struct {
	Output struct {
		ID                string `xml:"IdentificadoTBAI"`
		Data              string `xml:"FechaRecepcion"`
		Status            string `xml:"Estado"`
		Description       string `xml:"Descripcion"`
		BasqueDescription string `xml:"Azalpena"` // Description, but in Basque
		Errors            []struct {
			Code              string `xml:"Codigo"`
			Description       string `xml:"Descripcion"`
			BasqueDescription string `xml:"Azalpena"`
		} `xml:"ResultadosValidacion"`
		CSV string `xml:"CSV"` // Secure Verification Code
	} `xml:"Salida"`
}

// GipuzkoaConn keeps all the connection details together for the Gipuzkoa region.
type GipuzkoaConn struct {
	client *resty.Client
}

func newGipuzkoa(env Environment, tlsConfig *tls.Config) *GipuzkoaConn {
	c := new(GipuzkoaConn)
	c.client = resty.New()

	switch env {
	case EnvironmentProduction:
		c.client.SetBaseURL(gipuzkoaProductionBaseURL)
	default:
		c.client.SetBaseURL(gipuzkoaTestingBaseURL)
	}

	c.client.SetTLSClientConfig(tlsConfig)
	c.client.SetDebug(debug())

	return c
}

// Post sends the complete TicketBAI document to the Gipuzkoa API.
func (c *GipuzkoaConn) Post(ctx context.Context, _ *bill.Invoice, doc *convert.TicketBAI) error {
	payload, err := doc.Bytes()
	if err != nil {
		return fmt.Errorf("generating payload: %w", err)
	}
	return c.post(ctx, gipuzkoaExecutePath, payload)
}

// Cancel will send a request to the Gipuzkoa API to cancel a previously issued document.
func (c *GipuzkoaConn) Cancel(ctx context.Context, _ *bill.Invoice, doc *convert.AnulaTicketBAI) error {
	payload, err := doc.Bytes()
	if err != nil {
		return fmt.Errorf("generating payload: %w", err)
	}
	return asCancelDuplicate(c.post(ctx, gipuzkoaCancelPath, payload))
}

func (c *GipuzkoaConn) post(ctx context.Context, path string, payload []byte) error {
	out := new(GipuzkoaResponse)
	req := c.client.R().
		SetContext(ctx).
		SetDebug(true).
		SetHeader("Content-Type", "application/xml").
		SetContentLength(true).
		SetBody(payload).
		SetResult(out)

	res, err := req.Post(path)
	if err != nil {
		return ErrConnection.withCause(err)
	}
	if res.StatusCode() != http.StatusOK {
		return ErrValidation.withCode(strconv.Itoa(res.StatusCode()))
	}

	if out.Output.Status != gipuzkoaStatusReceived {
		err := ErrValidation
		if len(out.Output.Errors) > 0 {
			e1 := out.Output.Errors[0]
			err = err.withMessage(e1.Description).withCode(e1.Code)
		}
		return err
	}

	return nil
}
