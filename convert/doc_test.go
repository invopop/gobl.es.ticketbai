package convert_test

import (
	"os"
	"testing"
	"time"

	"github.com/nbio/xml"

	"github.com/invopop/gobl.ticketbai/convert"
	"github.com/invopop/gobl.ticketbai/test"
	"github.com/invopop/gobl/addons/es/tbai"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvoiceConversion(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2022-02-01T04:00:00Z")
	require.NoError(t, err)
	role := convert.IssuerRoleThirdParty

	t.Run("fail when missing zone", func(t *testing.T) {
		inv := test.LoadInvoice("sample-invoice.json")

		_, err := convert.NewTicketBAI(inv, ts, role, l10n.Code(""))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "zone is required")
	})

	t.Run("should have the right version", func(t *testing.T) {
		inv := test.LoadInvoice("sample-invoice.json")
		invoice, err := convert.NewTicketBAI(inv, ts, role, convert.ZoneBI)

		require.NoError(t, err)
		assert.Equal(t, "1.2", invoice.Cabecera.IDVersionTBAI)
	})

	t.Run("should contain info about the supplier", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("sample-invoice.json")
		goblInvoice.Supplier.TaxID.Code = "X34789654"
		goblInvoice.Supplier.Name = "Fake Company SL"

		invoice, _ := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)

		assert.Equal(t, "Fake Company SL", invoice.Sujetos.Emisor.ApellidosNombreRazonSocial)
		assert.Equal(t, "X34789654", invoice.Sujetos.Emisor.NIF)
	})

	t.Run("should contain the issuer role code", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("sample-invoice.json")

		invoice, _ := convert.NewTicketBAI(goblInvoice, ts, convert.IssuerRoleCustomer, convert.ZoneBI)

		assert.Equal(t, "D", invoice.Sujetos.EmitidaPorTercerosODestinatario)
	})

	t.Run("should contain info about national customer", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("sample-invoice.json")
		goblInvoice.Customer.TaxID = &tax.Identity{Country: "ES", Code: "17654245G"}
		goblInvoice.Customer.Name = "Spanish Co SL"
		goblInvoice.Customer.Addresses[0].Code = "50250"
		goblInvoice.Customer.Addresses[0].PostOfficeBox = "PO-745"

		invoice, _ := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)

		assert.Equal(t, "17654245G", invoice.Sujetos.Destinatarios.IDDestinatario[0].NIF)
		assert.Equal(t, "Spanish Co SL", invoice.Sujetos.Destinatarios.IDDestinatario[0].ApellidosNombreRazonSocial)
		assert.Equal(t, "50250", invoice.Sujetos.Destinatarios.IDDestinatario[0].CodigoPostal)
		assert.Contains(t, invoice.Sujetos.Destinatarios.IDDestinatario[0].Direccion, "PO-745")
	})

	t.Run("EU customer tax ID is emitted as NIF-VAT (IDType 02)", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("sample-invoice.json")
		goblInvoice.Customer.TaxID = &tax.Identity{Country: "DE", Code: "DE123456789"}
		goblInvoice.Customer.Name = "EU Co GmbH"

		invoice, _ := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)

		assert.Equal(t, "DE", invoice.Sujetos.Destinatarios.IDDestinatario[0].IDOtro.CodigoPais)
		assert.Equal(t, "DE123456789", invoice.Sujetos.Destinatarios.IDDestinatario[0].IDOtro.ID)
		assert.Equal(t, "02", invoice.Sujetos.Destinatarios.IDDestinatario[0].IDOtro.IDType)
		assert.Equal(t, "EU Co GmbH", invoice.Sujetos.Destinatarios.IDDestinatario[0].ApellidosNombreRazonSocial)
	})

	t.Run("non-EU customer tax ID is emitted as Foreign Identity (IDType 04)", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("sample-invoice.json")
		goblInvoice.Customer.TaxID = &tax.Identity{Country: "GB", Code: "GB123456789"}
		goblInvoice.Customer.Name = "Abroad Co LLC"

		invoice, _ := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)

		assert.Equal(t, "GB", invoice.Sujetos.Destinatarios.IDDestinatario[0].IDOtro.CodigoPais)
		assert.Equal(t, "GB123456789", invoice.Sujetos.Destinatarios.IDDestinatario[0].IDOtro.ID)
		assert.Equal(t, "04", invoice.Sujetos.Destinatarios.IDDestinatario[0].IDOtro.IDType)
		assert.Equal(t, "Abroad Co LLC", invoice.Sujetos.Destinatarios.IDDestinatario[0].ApellidosNombreRazonSocial)
	})

	t.Run("should not include customer if no tax ID present", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("sample-invoice.json")
		goblInvoice.Customer.TaxID = nil
		goblInvoice.Customer.Identities = nil

		invoice, _ := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)

		assert.Empty(t, invoice.Sujetos.Destinatarios)
	})

	t.Run("should change the document type from the default (02) if stated", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("sample-invoice.json")
		goblInvoice.Customer.TaxID = nil
		goblInvoice.Customer.Identities = []*org.Identity{
			{
				Key:  org.IdentityKeyPassport,
				Code: "PP123456S",
			},
		}
		require.NoError(t, goblInvoice.Calculate())
		invoice, _ := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)

		dest := invoice.Sujetos.Destinatarios.IDDestinatario[0]
		assert.Equal(t, "03", dest.IDOtro.IDType)
		assert.Equal(t, "PP123456S", dest.IDOtro.ID)
		assert.Empty(t, dest.IDOtro.CodigoPais)
	})

	t.Run("should use identities when Spanish customer has tax ID without code", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("sample-invoice.json")
		goblInvoice.Customer.TaxID = &tax.Identity{Country: "ES"}
		goblInvoice.Customer.Identities = []*org.Identity{
			{Key: org.IdentityKeyPassport, Code: "PP123456S"},
		}
		require.NoError(t, goblInvoice.Calculate())
		invoice, _ := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)

		dest := invoice.Sujetos.Destinatarios.IDDestinatario[0]
		assert.Empty(t, dest.NIF)
		assert.Equal(t, "03", dest.IDOtro.IDType)
		assert.Equal(t, "PP123456S", dest.IDOtro.ID)
		assert.Equal(t, "ES", dest.IDOtro.CodigoPais)
	})

	t.Run("should use identity country as CodigoPais when no tax ID", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("sample-invoice.json")
		goblInvoice.Customer.TaxID = nil
		goblInvoice.Customer.Identities = []*org.Identity{
			{Country: "ES", Key: org.IdentityKeyPassport, Code: "PP123456S"},
		}
		require.NoError(t, goblInvoice.Calculate())
		invoice, _ := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)

		dest := invoice.Sujetos.Destinatarios.IDDestinatario[0]
		assert.Equal(t, "03", dest.IDOtro.IDType)
		assert.Equal(t, "PP123456S", dest.IDOtro.ID)
		assert.Equal(t, "ES", dest.IDOtro.CodigoPais)
	})

	t.Run("es-tbai-identity-type extension is honoured directly", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("sample-invoice.json")
		goblInvoice.Customer.TaxID = nil
		goblInvoice.Customer.Identities = []*org.Identity{
			{
				Country: "CH",
				Code:    "CH-XYZ-001",
				Ext: tax.ExtensionsOf(cbc.CodeMap{
					tbai.ExtKeyIdentityType: "06",
				}),
			},
		}
		invoice, _ := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)

		dest := invoice.Sujetos.Destinatarios.IDDestinatario[0]
		assert.Equal(t, "06", dest.IDOtro.IDType)
		assert.Equal(t, "CH-XYZ-001", dest.IDOtro.ID)
		assert.Equal(t, "CH", dest.IDOtro.CodigoPais)
	})

	t.Run("es-tbai-identity-type extension honours each L7 code", func(t *testing.T) {
		for _, code := range []cbc.Code{"02", "03", "04", "05", "06"} {
			t.Run("code "+code.String(), func(t *testing.T) {
				goblInvoice := test.LoadInvoice("sample-invoice.json")
				goblInvoice.Customer.TaxID = nil
				goblInvoice.Customer.Identities = []*org.Identity{
					{
						Country: "FR",
						Code:    cbc.Code("ID-" + code.String()),
						Ext: tax.ExtensionsOf(cbc.CodeMap{
							tbai.ExtKeyIdentityType: code,
						}),
					},
				}
				invoice, _ := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)

				dest := invoice.Sujetos.Destinatarios.IDDestinatario[0]
				assert.Equal(t, code.String(), dest.IDOtro.IDType)
				assert.Equal(t, "ID-"+code.String(), dest.IDOtro.ID)
				assert.Equal(t, "FR", dest.IDOtro.CodigoPais)
			})
		}
	})

	t.Run("invoice-es-ch-tbai-other-identity fixture emits IDType=06", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("invoice-es-ch-tbai-other-identity.json")
		invoice, err := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)
		require.NoError(t, err)

		dest := invoice.Sujetos.Destinatarios.IDDestinatario[0]
		assert.Empty(t, dest.NIF)
		require.NotNil(t, dest.IDOtro)
		assert.Equal(t, "CH", dest.IDOtro.CodigoPais)
		assert.Equal(t, "06", dest.IDOtro.IDType)
		assert.Equal(t, "CH-OTHER-9001", dest.IDOtro.ID)
	})

	t.Run("identity with unknown key and no extension is skipped", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("sample-invoice.json")
		goblInvoice.Customer.TaxID = nil
		goblInvoice.Customer.Identities = []*org.Identity{
			{
				Key:  cbc.Key("unrecognised"),
				Code: "X-001",
			},
		}
		require.NoError(t, goblInvoice.Calculate())
		invoice, _ := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)

		assert.Nil(t, invoice.Sujetos.Destinatarios)
	})

	t.Run("should allow having no customer (useful for simplied invoices)", func(t *testing.T) {
		goblInvoice := test.LoadInvoice("sample-invoice.json")
		goblInvoice.Customer = nil

		invoice, _ := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)

		assert.Nil(t, invoice.Sujetos.Destinatarios)
	})

	t.Run("fail when charges are present since they aren't supported", func(t *testing.T) {
		inv := test.LoadInvoice("sample-invoice.json")
		inv.Lines[0].Charges = []*bill.LineCharge{{Amount: num.MakeAmount(100, 2)}}

		_, err := convert.NewTicketBAI(inv, ts, role, convert.ZoneBI)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "charges are not supported")

		inv.Lines[0].Charges = nil
		inv.Charges = []*bill.Charge{{Amount: num.MakeAmount(100, 2)}}

		_, err = convert.NewTicketBAI(inv, ts, role, convert.ZoneBI)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "charges are not supported")
	})
}

func TestDocumentParsing(t *testing.T) {
	path := test.Path("test", "data", "out", "sample-invoice.xml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	in := new(convert.TicketBAI)
	err = xml.Unmarshal(data, in)
	require.NoError(t, err)

	assert.Equal(t, "1089.00", in.Factura.DatosFactura.ImporteTotalFactura)
	assert.Equal(t, "900.00", in.Factura.TipoDesglose.DesgloseFactura.Sujeta.NoExenta.DetalleNoExenta[0].DesgloseIVA.DetalleIVA[0].BaseImponible)
	assert.Equal(t, "AQAB", in.Signature.KeyInfo.KeyValue.RSA.Exponent)
}
