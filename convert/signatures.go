package convert

import (
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/xmldsig"
	"github.com/invopop/xmldsig/profiles/facturae"
)

// SignerRoles defined in the TicketBAI spec
const (
	XAdESSupplier   xmldsig.XAdESSignerRole = "Supplier"
	XAdESCustomer   xmldsig.XAdESSignerRole = "Customer"
	XAdESThirdParty xmldsig.XAdESSignerRole = "Thirdparty"

	XAdESPolicyURLZoneBI string = "https://www.batuz.eus/fitxategiak/batuz/ticketbai/sinadura_elektronikoaren_zehaztapenak_especificaciones_de_la_firma_electronica_v1_0.pdf"
	XAdESPolicyURLZoneSS string = "https://www.gipuzkoa.eus/ticketbai/sinadura"
	XAdESPolicyURLZoneVI string = "https://ticketbai.araba.eus/tbai/sinadura/"
)

func newSignature(doc any, docID string, zone l10n.Code, role IssuerRole, cert *xmldsig.Certificate, opts ...xmldsig.Option) (*xmldsig.Signature, error) {
	data, err := toBytesCanonical(doc)
	if err != nil {
		return nil, err
	}

	opts = append(opts,
		xmldsig.WithDocID(docID),
		xmldsig.WithXMLDSigConfig(XMLDSigConfig()),
		xmldsig.WithXAdESConfig(XAdESConfig(zone, signerRole(role))),
		xmldsig.WithCertificate(cert),
		xmldsig.WithNamespace("T", ticketBAIEmisionNamespace), // default
	)

	return xmldsig.Sign(data, opts...)
}

// SignatureValue provides quick access to the XML signatures final value,
// useful for inclusion in the database.
func (doc *TicketBAI) SignatureValue() string {
	if doc.Signature == nil {
		return ""
	}
	return doc.Signature.Value.Value
}

// SignatureValue provides quick access to the XML signatures final value.
func (doc *AnulaTicketBAI) SignatureValue() string {
	if doc.Signature == nil {
		return ""
	}
	return doc.Signature.Value.Value
}

func signerRole(role IssuerRole) xmldsig.XAdESSignerRole {
	switch role {
	case IssuerRoleSupplier:
		return XAdESSupplier
	case IssuerRoleCustomer:
		return XAdESCustomer
	case IssuerRoleThirdParty:
		return XAdESThirdParty
	default:
		return ""
	}
}

// XMLDSigConfig returns the XMLDSig configuration required by the TicketBAI spec
func XMLDSigConfig() xmldsig.XMLDSigConfig {
	dsigCfg := facturae.XMLDSigConfig() // Based on facturae profile
	dsigCfg.OmitDataCanonicalizationTransform = true
	return dsigCfg
}

// XAdESConfig returns the policies configuration for signing a TicketBAI doc
func XAdESConfig(zone l10n.Code, role xmldsig.XAdESSignerRole) xmldsig.XAdESConfig {
	switch zone {
	case ZoneBI: // Bizkaia
		return facturae.XAdESConfig( // Based on facturae profile
			xmldsig.XAdESConfig{
				Role:        role,
				Description: "",
				Policy: &xmldsig.XAdESPolicyConfig{
					URL:         XAdESPolicyURLZoneBI,
					Description: "",
					Algorithm:   xmldsig.AlgDSigRSASHA256,
					Hash:        "Quzn98x3PMbSHwbUzaj5f5KOpiH0u8bvmwbbbNkO9Es=",
				},
			},
		)
	case ZoneSS: // Gipuzkoa
		return facturae.XAdESConfig( // Based on facturae profile
			xmldsig.XAdESConfig{
				Role:        role,
				Description: "",
				Policy: &xmldsig.XAdESPolicyConfig{
					URL:         XAdESPolicyURLZoneSS,
					Description: "",
					Algorithm:   xmldsig.AlgDSigRSASHA256,
					Hash:        "vSe1CH7eAFVkGN0X2Y7Nl9XGUoBnziDA5BGUSsyt8mg=",
				},
			},
		)
	case ZoneVI: // Araba
		return facturae.XAdESConfig( // Based on facturae profile
			xmldsig.XAdESConfig{
				Role:        role,
				Description: "",
				Policy: &xmldsig.XAdESPolicyConfig{
					URL:         XAdESPolicyURLZoneVI,
					Description: "",
					Algorithm:   xmldsig.AlgDSigRSASHA256,
					Hash:        "4Vk3uExj7tGn9DyUCPDsV9HRmK6KZfYdRiW3StOjcQA=",
				},
			},
		)
	}
	return xmldsig.XAdESConfig{}
}
