package convert_test

import (
	"testing"
	"time"

	"github.com/invopop/gobl.es.ticketbai/convert"
	"github.com/invopop/gobl.es.ticketbai/test"
	"github.com/invopop/xmldsig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignatureGeneration(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2022-02-01T04:00:00Z")
	require.NoError(t, err)

	cert, err := xmldsig.LoadCertificate(test.Path("test", "certs", "EntitateOrdezkaria_RepresentanteDeEntidad.p12"), "IZDesa2025")
	require.NoError(t, err)

	t.Run("should set the proper signer role", func(t *testing.T) {
		role := convert.IssuerRoleCustomer

		goblInvoice := test.LoadInvoice("sample-invoice.json")
		invoice, err := convert.NewTicketBAI(goblInvoice, ts, role, convert.ZoneBI)
		require.NoError(t, err)

		err = invoice.Sign("TEST", cert, role, convert.ZoneBI)
		require.NoError(t, err)

		assert.Equal(t,
			string(convert.XAdESCustomer),
			invoice.Signature.Object.QualifyingProperties.SignedProperties.
				SignedSignatureProperties.SignerRole.ClaimedRoles.ClaimedRole[0],
		)
	})

}
