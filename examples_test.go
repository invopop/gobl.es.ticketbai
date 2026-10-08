package ticketbai_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ticketbai "github.com/invopop/gobl.es.ticketbai"
	"github.com/invopop/gobl.es.ticketbai/convert"
	"github.com/invopop/gobl.es.ticketbai/internal/gateways"
	"github.com/invopop/gobl.es.ticketbai/test"
	"github.com/invopop/xmldsig"
	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/xsd"
	"github.com/stretchr/testify/require"
)

const (
	msgMissingOutFile    = "output file %s missing, run tests with `--update` flag to create"
	msgUnmatchingOutFile = "output file %s does not match, run tests with `--update` flag to update"
)

func TestXMLGeneration(t *testing.T) {
	schema, err := xsd.NewCompiler().CompileFile(
		context.Background(),
		test.Path("test", "schema", "schema.xsd"),
	)
	require.NoError(t, err)

	examples, err := lookupExamples()
	require.NoError(t, err)

	tbai, err := loadTBAIClient()
	require.NoError(t, err)

	for _, example := range examples {
		name := fmt.Sprintf("should convert %s example file successfully", example)

		t.Run(name, func(t *testing.T) {
			data, err := convertExample(tbai, example)
			require.NoError(t, err)

			// Validate against the TicketBAI XSD on every run so
			// CI catches schema regressions even without --update.
			require.NoError(t, validateDoc(schema, data))

			outPath := test.Path("test", "data", "out",
				strings.TrimSuffix(example, ".json")+".xml",
			)

			if *test.UpdateOut {
				err = os.WriteFile(outPath, data, 0644)
				require.NoError(t, err)
				return
			}

			expected, err := os.ReadFile(outPath)

			require.False(t, os.IsNotExist(err), msgMissingOutFile, filepath.Base(outPath))
			require.NoError(t, err)
			require.Equal(t, string(expected), string(data), msgUnmatchingOutFile, filepath.Base(outPath))
		})
	}
}

func loadTBAIClient() (*ticketbai.Client, error) {
	pass, err := os.ReadFile(
		test.Path("test", "certs", "EntitateOrdezkaria_RepresentanteDeEntidad_pin.txt"),
	)
	if err != nil {
		return nil, err
	}

	cert, err := xmldsig.LoadCertificate(
		test.Path("test", "certs", "EntitateOrdezkaria_RepresentanteDeEntidad.p12"),
		string(pass),
	)
	if err != nil {
		return nil, err
	}

	ts, err := time.Parse(time.RFC3339, "2022-02-01T04:00:00Z")
	if err != nil {
		return nil, err
	}

	return ticketbai.New(&ticketbai.Software{
		Licenses: ticketbai.Licenses{
			gateways.EnvironmentSandbox: {
				ticketbai.ZoneBI: "My License",
			},
		},
		NIF:     "12345678A",
		Name:    "My Software",
		Version: "1.0",
	},
		ticketbai.ZoneBI,
		ticketbai.WithCertificate(cert),
		ticketbai.WithCurrentTime(ts),
		ticketbai.WithThirdPartyIssuer(),
	)
}

func lookupExamples() ([]string, error) {
	examples, err := filepath.Glob(test.Path("test", "data", "*.json"))
	if err != nil {
		return nil, err
	}

	for i, example := range examples {
		examples[i] = filepath.Base(example)
	}

	return examples, nil
}

func convertExample(tc *ticketbai.Client, example string) ([]byte, error) {
	env := test.LoadEnvelope(example)

	td, err := tc.Convert(env)
	if err != nil {
		return nil, err
	}

	err = tc.Fingerprint(td, &convert.ChainData{
		Series:    "AF",
		Code:      "1234567890",
		IssueDate: "01-01-2021",
		Signature: strings.Repeat("1234567890", 20),
	})
	if err != nil {
		return nil, err
	}

	err = tc.Sign(td, env)
	if err != nil {
		return nil, err
	}

	return td.BytesIndent()
}

// validateDoc parses the generated XML and validates it against the
// pre-compiled TicketBAI XSD. The schema's wrapper file pre-imports the
// W3C xmldsig schema from a local copy in test/schema/, so no network
// access is needed at compile time. Individual validation errors are
// collected so failures surface every issue, not just "validation failed".
func validateDoc(schema *xsd.Schema, data []byte) error {
	ctx := context.Background()
	doc, err := helium.NewParser().Parse(ctx, data)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	collector := helium.NewErrorCollector(ctx, helium.ErrorLevelNone)
	if err := xsd.NewValidator(schema).ErrorHandler(collector).Validate(ctx, doc); err != nil {
		msgs := make([]string, 0, len(collector.Errors()))
		for _, e := range collector.Errors() {
			msgs = append(msgs, e.Error())
		}
		if len(msgs) == 0 {
			return fmt.Errorf("validate: %w", err)
		}
		return fmt.Errorf("validate:\n  %s", strings.Join(msgs, "\n  "))
	}
	return nil
}
