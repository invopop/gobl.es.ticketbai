package convert

import (
	"strings"

	"github.com/invopop/gobl/addons/es/tbai"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
)

// Sujetos contains invoice parties info
type Sujetos struct {
	Emisor                          *Emisor
	Destinatarios                   *Destinatarios
	EmitidaPorTercerosODestinatario string
}

// Emisor contains info about the invoice supplier
type Emisor struct {
	NIF                        string
	ApellidosNombreRazonSocial string
}

// Destinatarios contains info about the invoice customers,
// Tickebai allows up to 100 customers but GOBL only allows one
// per invoice
type Destinatarios struct {
	IDDestinatario []*IDDestinatario
}

// IDDestinatario contains info about a single customer
type IDDestinatario struct {
	NIF                        string  `xml:",omitempty"`
	IDOtro                     *IDOtro `xml:",omitempty"`
	ApellidosNombreRazonSocial string
	CodigoPostal               string `xml:",omitempty"`
	Direccion                  string `xml:",omitempty"`
}

// IDOtro identifies a customer for a contry other than Spain
type IDOtro struct {
	CodigoPais string `xml:",omitempty"`
	IDType     string
	ID         string
}

func newEmisor(party *org.Party) *Emisor {
	return &Emisor{
		NIF:                        party.TaxID.Code.String(),
		ApellidosNombreRazonSocial: party.Name,
	}
}

func newDestinatario(party *org.Party) *IDDestinatario {
	d := &IDDestinatario{
		ApellidosNombreRazonSocial: party.Name,
	}

	if party.TaxID != nil && party.TaxID.Country == "ES" && party.TaxID.Code != "" {
		d.NIF = party.TaxID.Code.String()
	} else {
		d.IDOtro = otherIdentity(party)
	}
	if d.NIF == "" && d.IDOtro == nil {
		// Assume this is a B2C operation.
		return nil
	}

	if len(party.Addresses) > 0 && party.Addresses[0].Code != "" {
		d.CodigoPostal = party.Addresses[0].Code.String()
		d.Direccion = formatAddress(party.Addresses[0])
	}

	return d
}

func otherIdentity(party *org.Party) *IDOtro {
	if party.TaxID != nil && party.TaxID.Code != "" {
		country := party.TaxID.Country.String()
		idType := taxIDType(party.TaxID.Country)
		id := party.TaxID.Code.String()
		if idType == tbai.ExtCodeIdentityTypeVAT.String() && !strings.HasPrefix(id, country) {
			id = country + id
		}
		return &IDOtro{
			CodigoPais: country,
			IDType:     idType,
			ID:         id,
		}
	}
	id := org.IdentityForExtKey(party.Identities, tbai.ExtKeyIdentityType)
	if id == nil || id.Code == "" {
		return nil
	}
	oid := &IDOtro{
		IDType: id.Ext.Get(tbai.ExtKeyIdentityType).String(),
		ID:     id.Code.String(),
	}
	switch {
	case id.Country != "":
		oid.CodigoPais = id.Country.String()
	case party.TaxID != nil:
		oid.CodigoPais = party.TaxID.Country.String()
	}
	return oid
}

// taxIDType maps a non-Spanish customer tax ID to its TicketBAI IDType:
// EU members → NIF-VAT (02), others → foreign document (04). The gateway
// rejects non-EU tax IDs sent as 02 with B4_2000013.
func taxIDType(country l10n.TaxCountryCode) string {
	if l10n.Union(l10n.EU).HasMember(country.Code()) {
		return tbai.ExtCodeIdentityTypeVAT.String()
	}
	return tbai.ExtCodeIdentityTypeForeign.String()
}

func partyCountry(party *org.Party) l10n.TaxCountryCode {
	if party == nil {
		return ""
	}
	if party.TaxID != nil && party.TaxID.Country != "" {
		return party.TaxID.Country
	}
	for _, id := range party.Identities {
		if id != nil && id.Country != "" {
			return l10n.TaxCountryCode(id.Country)
		}
	}
	return ""
}
