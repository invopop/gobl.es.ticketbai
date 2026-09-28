package gateways

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/go-resty/resty/v2"
	"github.com/invopop/gobl.ticketbai/convert"
	"github.com/invopop/gobl/bill"
)

// Integration Guide for Araba region:
//
//  * Testing - https://web.araba.eus/documents/105044/5608600/Gu%C3%ADa+entorno+pruebas+TicketBAI+-+castellano.pdf/b74f0114-0b35-73c4-5c10-7741ad393658?t=1642408082727
//  * Production - https://web.araba.eus/documents/105044/5608600/Anexo+IV+TicketBAI.pdf/fdf77a97-cbdb-8655-7e3f-b87c046da48e?t=1652787434485
//

const (
	// Requests
	arabaProductionBaseURL = "https://ticketbai.araba.eus"
	arabaTestingBaseURL    = "https://pruebas-ticketbai.araba.eus"

	arabaExecutePath = "/TicketBAI/v1/facturas/"
	arabaCancelPath  = "/TicketBAI/v1/anulaciones/"
)

const (
	arabaStatusReceived = "00" // looks good.
	arabaStatusRejected = "01" // there are errors that need fixing.
)

// ArabaResponse defines the response fields from the Araba region.
type ArabaResponse struct {
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

// Err converts a response that was not accepted into a structured error. The
// code and description of the first validation result are used when present,
// otherwise the general description of the outcome.
func (r *ArabaResponse) Err() *Error {
	if len(r.Output.Errors) > 0 {
		e1 := r.Output.Errors[0]
		return ErrValidation.withCode(e1.Code).withMessage(e1.Description)
	}
	return ErrValidation.withMessage(r.Output.Description)
}

// ArabaConn keeps all the connection details together for the Araba region.
type ArabaConn struct {
	client *resty.Client
}

func newAraba(env Environment, tlsConfig *tls.Config) *ArabaConn {
	c := new(ArabaConn)
	c.client = resty.New()

	switch env {
	case EnvironmentProduction:
		c.client.SetBaseURL(arabaProductionBaseURL)
	default:
		c.client.SetBaseURL(arabaTestingBaseURL)
	}

	c.client.SetTLSClientConfig(tlsConfig)
	c.client.SetDebug(debug())

	return c
}

// Post sends the complete TicketBAI document to the Araba API.
func (c *ArabaConn) Post(ctx context.Context, _ *bill.Invoice, doc *convert.TicketBAI) error {
	payload, err := doc.Bytes()
	if err != nil {
		return fmt.Errorf("generating payload: %w", err)
	}
	return c.post(ctx, arabaExecutePath, payload)
}

// Cancel will send a request to the Araba API to cancel a previously issued document.
func (c *ArabaConn) Cancel(ctx context.Context, _ *bill.Invoice, doc *convert.AnulaTicketBAI) error {
	payload, err := doc.Bytes()
	if err != nil {
		return fmt.Errorf("generating payload: %w", err)
	}
	return asCancelDuplicate(c.post(ctx, arabaCancelPath, payload))
}

func (c *ArabaConn) post(ctx context.Context, path string, payload []byte) error {
	out := new(ArabaResponse)
	req := c.client.R().
		SetContext(ctx).
		SetDebug(true).
		// The charset must be declared explicitly. Without it the remote end
		// decodes the body as ISO-8859-1, which mangles any non-ASCII character
		// and breaks the XAdES SignedProperties digest.
		SetHeader("Content-Type", "application/xml; charset=UTF-8").
		SetContentLength(true).
		SetBody(payload).
		SetResult(out).
		// Rejections may also be sent alongside an error status code, and
		// will only be parsed if we define them as the error result.
		SetError(out)

	res, err := req.Post(path)
	if err != nil {
		return ErrConnection.withCause(err)
	}
	// Always prefer the parsed response, it describes the problem in much
	// more detail than the status code alone.
	if out.Output.Status != "" && out.Output.Status != arabaStatusReceived {
		return out.Err()
	}
	if res.StatusCode() != http.StatusOK {
		return statusError(res)
	}

	return nil
}
