// Package ticketbai provides a client for generating and sending TicketBAI
// documents to the different regional services in the Basque Country.
package ticketbai

import (
	"context"
	"time"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl.ticketbai/convert"
	"github.com/invopop/gobl.ticketbai/internal/gateways"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/xmldsig"
	"github.com/nbio/xml"
)

// Expose zone codes for external use.
const (
	ZoneBI l10n.Code = convert.ZoneBI // Bizkaia
	ZoneSS l10n.Code = convert.ZoneSS // Gipuzkoa
	ZoneVI l10n.Code = convert.ZoneVI // Araba
)

// Client provides the main interface to the TicketBAI package.
type Client struct {
	software   *Software
	zone       l10n.Code
	cert       *xmldsig.Certificate
	env        gateways.Environment
	issuerRole convert.IssuerRole
	curTime    time.Time
	gw         gateways.Connection
}

// Option is used to configure the client.
type Option func(*Client)

// WithCertificate defines the signing certificate to use when producing the
// TicketBAI document.
func WithCertificate(cert *xmldsig.Certificate) Option {
	return func(c *Client) {
		c.cert = cert
	}
}

// WithCurrentTime defines the current time to use when generating the TicketBAI
// document. Useful for testing.
func WithCurrentTime(curTime time.Time) Option {
	return func(c *Client) {
		c.curTime = curTime
	}
}

// WithConnection defines a new gateway connection to use for the client.
// Useful for testing and mocking the connection responses.
func WithConnection(conn gateways.Connection) Option {
	return func(c *Client) {
		c.gw = conn
	}
}

// WithSupplierIssuer set the issuer type to supplier. To be used when the
// invoice's supplier, using their own certificate, is issuing the document.
func WithSupplierIssuer() Option {
	return func(c *Client) {
		c.issuerRole = convert.IssuerRoleSupplier
	}
}

// WithCustomerIssuer set the issuer type to customer. To be used when the
// invoice's supplier, using their own certificate, is issuing the document.
func WithCustomerIssuer() Option {
	return func(c *Client) {
		c.issuerRole = convert.IssuerRoleCustomer
	}
}

// WithThirdPartyIssuer set the issuer type to third party. To be used when the
// an authorised third party, using their own certificate, is issuing the
// document on behalf of the invoice's supplier.
func WithThirdPartyIssuer() Option {
	return func(c *Client) {
		c.issuerRole = convert.IssuerRoleThirdParty
	}
}

// InProduction defines the connection to use the production environment.
func InProduction() Option {
	return func(c *Client) {
		c.env = gateways.EnvironmentProduction
	}
}

// InSandbox defines the connection to use the testing environment.
func InSandbox() Option {
	return func(c *Client) {
		c.env = gateways.EnvironmentSandbox
	}
}

// Licenses stores the licenses for the different zones and environments.
type Licenses map[gateways.Environment]map[l10n.Code]string

// Software defines the details about the software that is using this library to
// generate TicketBAI documents. These details are included in the final
// document.
type Software struct {
	Licenses    Licenses
	NIF         string
	Name        string
	CompanyName string
	Version     string
}

// New creates a new TicketBAI client with shared software and configuration
// options for creating and sending new documents.
func New(software *Software, zone l10n.Code, opts ...Option) (*Client, error) {
	c := new(Client)
	c.software = software
	c.zone = zone

	// Set default values that can be overwritten by the options
	c.env = gateways.EnvironmentSandbox
	c.issuerRole = convert.IssuerRoleSupplier

	for _, opt := range opts {
		opt(c)
	}

	if c.cert == nil {
		return c, nil
	}

	if c.gw == nil {
		var err error
		c.gw, err = gateways.New(c.env, c.zone, c.cert)
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

// Post will send the document to the TicketBAI gateway.
func (c *Client) Post(ctx context.Context, env *gobl.Envelope, d *convert.TicketBAI) error {
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return ErrValidation.withMessage("only invoices are supported")
	}
	if err := c.gw.Post(ctx, inv, d); err != nil {
		return newErrorFrom(err)
	}
	return nil
}

// Cancel will send the cancel document in the TicketBAI gateway.
func (c *Client) Cancel(ctx context.Context, env *gobl.Envelope, d *convert.AnulaTicketBAI) error {
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return ErrValidation.withMessage("only invoices are supported")
	}
	return c.gw.Cancel(ctx, inv, d)
}

// ParseDocument will parse the XML data into a TicketBAI document.
func ParseDocument(data []byte) (*convert.TicketBAI, error) {
	d := new(convert.TicketBAI)
	if err := xml.Unmarshal(data, d); err != nil {
		return nil, err
	}
	return d, nil
}

// ParseCancelDocument will parse the XML data into a Cancel TicketBAI document.
func ParseCancelDocument(data []byte) (*convert.AnulaTicketBAI, error) {
	d := new(convert.AnulaTicketBAI)
	if err := xml.Unmarshal(data, d); err != nil {
		return nil, err
	}
	return d, nil
}

// CurrentTime returns the current time to use when generating
// the TicketBAI document.
func (c *Client) CurrentTime() time.Time {
	if !c.curTime.IsZero() {
		return c.curTime
	}
	return time.Now()
}

// Zone returns the zone for this client.
func (c *Client) Zone() l10n.Code {
	return c.zone
}

// Sandbox returns true if the client is using the sandbox environment.
func (c *Client) Sandbox() bool {
	return c.env == gateways.EnvironmentSandbox
}

func (c *Client) buildSoftware() *convert.Software {
	return &convert.Software{
		License: c.software.Licenses[c.env][c.zone],
		NIF:     c.software.NIF,
		Name:    c.software.Name,
		Version: c.software.Version,
	}
}
