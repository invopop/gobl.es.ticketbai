# GOBL to TicketBAI

Go library to convert [GOBL](https://github.com/invopop/gobl) invoices into TicketBAI declarations and send them to the Basque Country web services.

This library assumes that clients will handle a local database of previous invoices in order to comply with the local requirements of chaining all invoices together.

Copyright [Invopop Ltd.](https://invopop.com) 2023. Released publicly under the [GNU Affero General Public License v3.0](LICENSE). For commercial licenses please contact the [dev team at invopop](mailto:dev@invopop.com). For contributions to this library to be accepted, we will require you to accept transferring your copyright to Invopop Ltd.

## Source

The basque country is split into 4 "foral" tax agencies. 3 of those tax agencies decided to adopt the TicketBAI format. Each of the three agencies has their own set of documentation and integration definitions.

Links to key information for each agency are described in the following subchapters.

### Bizkaia

- BOB: https://www.batuz.eus/fitxategiak/batuz/normativa/2020%2009%2011%20ORDEN%20FORAL%201482-2020,%20de%209%20de%20septiembre.pdf?hash=929e00a8bbd774ee911841d36e3301e2
- XSD basic TicketBAI request: https://www.batuz.eus/fitxategiak/batuz/ticketbai/ticketBaiV1-2.xsd
- XSD complete list: https://www.batuz.eus/fitxategiak/Batuz/LROE/esquemas/Esquemas%20XSD.7z
- Ley IVA Foral, con datos útiles sobre exclusiones (a partir de página 24): https://www.bizkaia.eus/ogasuna/zerga_arautegia/indarreko_arautegia/pdf/ca_7_1994.pdf
- FAQ Gipuzkoa: https://www.gipuzkoa.eus/es/web/ogasuna/ticketbai/faq (download the PDF from that page, it's much easier to read.)

## Usage

### Go Package

You must have first created a GOBL Envelope containing an Invoice that you'd like to send to one of the TicketBAI web services.

For the document to be converted, the supplier contained in the invoice should have a "Tax ID" with the country set to `ES`.

TicketBAI is used by three different tax agencies (Haciendas Forales), each of which has their own API and specific requirements. The `es-tbai-region` extension defined in the GOBL Invoice's `tax` property is used to set the determine the correct API to utilize. This will be set automatically in most cases.

The following is an example of how the GOBL TicketBAI package could be used:

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/invopop/gobl"
	ticketbai "github.com/invopop/gobl.ticketbai"
	"github.com/invopop/xmldsig"
)

func main() {
	ctx := context.Background()

	// Load sample envelope:
	data, _ := os.ReadFile("./test/data/sample-invoice.json")

	env := new(gobl.Envelope)
	if err := json.Unmarshal(data, env); err != nil {
		panic(err)
	}
	zone := ticketbai.ZoneFor(env)

	// Prepare software configuration:
	soft := &ticketbai.Software{
		License: "XYZ",        // provided by tax agency
		NIF:     "B123456789", // Software company's tax code
		Name:    "Invopop",    // Name of application
		Version: "v0.1.0",     // Software version
	}

	// Load sample certificate:
	cert, err := xmldsig.LoadCertificate(
		"./test/certs/EnpresaZigilua_SelloDeEmpresa.p12", "IZDesa2025")
	if err != nil {
		panic(err)
	}

	// Instantiate the TicketBAI client with sofrward config
	// and specific zone.
	tc, err := ticketbai.New(soft, zone,
		ticketbai.WithCertificate(cert), // Use the certificate previously loaded
		ticketbai.WithSupplierIssuer(),  // The issuer is the invoice's supplier
		ticketbai.InTesting(),           // Use the tax agency testing environment
	)
	if err != nil {
		panic(err)
	}

	// Create a new TBAI document:
	doc, err := tc.Convert(env)
	if err != nil {
		panic(err)
	}

	// Create the document fingerprint
	// Assume here that we don't have a previous chain data object.
	if err = tc.Fingerprint(doc, nil); err != nil {
		panic(err)
	}

	// Sign the document:
	if err := tc.Sign(doc, env); err != nil {
		panic(err)
	}

	// Create the XML output
	bytes, err := doc.BytesIndent()
	if err != nil {
		panic(err)
	}

	// Do something with the output, you probably want to store
	// it somewhere.
	fmt.Println("Document created:\n", string(bytes))

	// Grab and persist the Chain Data somewhere so you can use this
	// for the next call to the Fingerprint method.
	cd := doc.ChainData()

	// Send to TicketBAI, if rejected, you'll want to fix any
	// issues and send in a new XML document. The original
	// version should not be modified.
	if err := tc.Post(ctx, doc); err != nil {
		panic(err)
	}

}
```

## Command Line

The GOBL TicketBAI package tool also includes a command line helper. You can find pre-built [gobl.cfdi binaries](https://github.com/invopop/gobl.ticketbai/releases) in the github repository, or install manually in your Go environment with:

```bash
go install github.com/invopop/gobl.ticketbai
```

We recommend using a `.env` file to prepare configuration settings, although all parameters can be set using command line flags. Heres an example:

```
CERTIFICATE_PATH="./test/certs/EntitateOrdezkaria_RepresentanteDeEntidad.p12"
CERTIFICATE_PASSWORD=IZDesa2025
SOFTWARE_COMPANY_NIF=A99800005
SOFTWARE_COMPANY_NAME="SOFTWARE GARANTE TICKETBAI PRUEBA"
SOFTWARE_NAME="Invopop"
SOFTWARE_LICENSE="TBAIBI00000000PRUEBA" # BI & SS
SOFTWARE_VERSION="1.0"
```

The `SOFTWARE_*` values above are the public test identity published by Batuz alongside the sandbox licence `TBAIBI00000000PRUEBA`. Using any other combination with this licence will be rejected.

To convert a document to XML, run:

```bash
gobl.ticketbai convert ./test/data/sample-invoice.json
```

To submit to the tax agency testing environment:

```bash
gobl.ticketbai send ./test/data/sample-invoice.json
```

## Limitations

- Tickebai allows more than one customer per invoice, but GOBL only has one possible customer.

- Invoices should have a note of type general that will be used as a general description of the invoice. If an invoice is missing this info, it will be rejected with an error.

- GOBL's corrective invoices aren't supported at the moment. Only credit and debit notes are, and they are converted into "Facturas Rectificativas por Diferencias" with either positive or inverted quantities depending on whether it is a debit or a credit note.

## Bizkaia: Modelo 140 vs Modelo 240

Bizkaia's LROE / Batuz system uses two different registers depending on the type of issuer:

- **Modelo 240** — for legal entities (_persona jurídica_, NIF/CIF starting with a letter, e.g. `B64847106`).
- **Modelo 140** — for individuals (_persona física_, DNI / NIE / non-resident IDs, e.g. `12345678Z`).

The library detects which model to use automatically from the supplier's tax identity, using GOBL's `es.TaxIdentityKey` helper. No configuration is required.

When the supplier is an individual in Bizkaia, their IAE-style activity code (_Epígrafe_) must be set on the supplier's `ext` map under the `es-tbai-bi-activity` key. This value is published in the `<Renta>` block of the Modelo 140 LROE submission. Reference:

- Activity codes (Epígrafes): https://www.batuz.eus/fitxategiak/batuz/lroe/batuz_lroe_lista_epigrafes_v1_0_4.xlsx
- Modelo 140 specification: https://www.batuz.eus/fitxategiak/batuz/lroe/lroe_140_v_1_0.pdf

```json
"supplier": {
  "name": "Ana Fernández García",
  "tax_id": { "country": "ES", "code": "12345678Z" },
  "ext": { "es-tbai-bi-activity": "722300" }
}
```

Álava (`VI`) and Gipuzkoa (`SS`) are unaffected — their gateways do not expose model selection.

## Tags, Keys and Extensions

In order to provide the supplier specific data required by TicketBAI, invoices need to include a bit of extra data. We've managed to simplify these into specific cases.

### Tax Tags

Invoice tax tags can be added to invoice documents in order to reflect a special situation. The following schemes are supported:

- `simplified-scheme` - a retailer operating under a simplified tax regime (regimen simplificado) that must indicate that all of their sales are under this scheme. This implies that all operations in the invoice will have the `OperacionEnRecargoDeEquivalenciaORegimenSimplificado` tag set to `S`.
- `reverse-charge` - B2B services or goods sold to a tax registered EU member who will pay VAT on the suppliers behalf. Implies that all items will be classified under the `TipoNoExenta` value of `S2`.
- `customer-rates` - B2C services, specifically for the EU digital goods act (2015) which imply local taxes will be applied. All items will specify the `DetalleNoSujeta` cause of `RL`.

## Tax Extensions

The following extension can be applied to each line tax:

- `es-tbai-product` – allows to correctly group the invoice's lines taxes in the TicketBAI breakdowns (a.k.a. desgloses). These are the valid values:

  - `services` - indicates that the product being sold is a service (as opposed to a physical good). Services are accounted in the `DesgloseTipoOperacion > PrestacionServicios` breakdown of invoices to foreign customers. By default, all items are considered services.
  - `goods` - indicates that the product being sold is a physical good. Products are accounted in the `DesgloseTipoOperacion > Entrega` breakdown of invoices to foreign customers.
  - `resale` - indicates that a line item is sold without modification from a provider under the Equalisation Charge scheme. (This implies that the `OperacionEnRecargoDeEquivalenciaORegimenSimplificado` tag will be set to `S`).

- `es-tbai-exemption` - identifies the specific TicketBAI reason code as to why taxes should not be applied to the line according to the whole set of exemptions or not-subject scenarios defined in the law. It has to be set along with the tax rate value of `exempt`. These are the valid values:
  - `E1` – Exenta por el artículo 20 de la Norma Foral del IVA
  - `E2` – Exenta por el artículo 21 de la Norma Foral del IVA
  - `E3` – Exenta por el artículo 22 de la Norma Foral del IVA
  - `E4` – Exenta por el artículo 23 y 24 de la Norma Foral del IVA
  - `E5` – Exenta por el artículo 25 de la Norma Foral del IVA
  - `E6` – Exenta por otra causa
  - `OT` – No sujeto por el artículo 7 de la Norma Foral de IVA / Otros supuestos
  - `RL` – No sujeto por reglas de localización (\*)

_(\*) As noted elsewhere, `RL` will be set automatically set in invoices using the `customer-rates` tax tag. It can also be set explicitly using the `es-tbai-exemption` extension in invoices not using that tag._

- `es-tbai-regime` - sets the `ClaveRegimenIvaOpTrascendencia` field per VAT/IGIC tax combo. Codes follow the TicketBAI XSD list (`01`–`17`, `51`–`53`). If not provided, GOBL fills it in during normalization from per-combo signals — `tax.KeyExport` → `02`, equivalence-surcharge rate → `51`, the invoice-level `simplified-scheme` tag → `52`, otherwise `01`. Set it explicitly when none of those defaults applies (e.g. travel agencies → `05`, cash accounting → `07`, OSS/IOSS → `17`); explicit values are always preserved.

- `es-tbai-identity-type` - sets the `IDType` value under `IDOtro` for the customer's identity (L7 list, codes `02`–`06`). Normalization maps `org.IdentityKeyPassport` → `03`, `IdentityKeyForeign` → `04`, `IdentityKeyResident` → `05`, `IdentityKeyOther` → `06`. Set it explicitly on an identity with no key (or to override). Spanish NIFs use the `NIF` field directly, and EU/non-EU tax IDs map to `IDOtro/IDType` `02`/`04` automatically.

### Use-Cases

Under what situations should the TicketBAI system be expected to function:

- B2B & B2C: regular national invoice with VAT. Operation with minimal data.
- B2B Provider to Retailer: Include equalisation surcharge VAT rates
- B2B Retailer: Same as regular invoice, except with invoice lines that include `ext[es-tbai-product] = resale` when the goods being provided are being sold without modification (recargo de equivalencia), very much related to the next point.
- B2B Retailer Simplified: Include the simplified scheme key. (This implies that the `OperacionEnRecargoDeEquivalenciaORegimenSimplificado` tag will be set to `S`).
- EU B2B: Reverse charge EU export, scheme: reverse-charge taxes calculated, but not applied to totals. By default all line items assumed to be services. Individual lines can use the `ext[es-tbai-product] = goods` value to identify when the line is a physical good. Operations like this are normally assigned the TipoNoExenta value of S2. If however the service or goods are exempt of tax, each line's tax `ext[exempt]` field can be used to identify a reason.
- EU B2C Digital Goods: use tax tag `customer-rates`, that applies VAT according to customer location. In TicketBAI, these cases are "not subject" to tax, and thus should have the cause RL (por reglas de localización).

## Test Data

Sample GOBL envelopes live in `test/data/*.json` and their converted XML
counterparts in `test/data/out/*.xml`. The fixtures use the Bizkaia sandbox
test entity (`S7836107H`, "ZIURTAPEN ZERBITZU ENPRESA-EMPRESA CERTIFICACION
Y SERVICIOS IZENPE SA") so they can be sent against Batuz' developer
endpoint without further editing.

### Regenerating the fixtures

If you edit any of the JSON inputs (for example to change the supplier),
the envelope `head.dig` and the converted XML need refreshing. Run:

```bash
go test . --update
```

That recomputes the digests in `test/data/*.json` and rewrites every
`test/data/out/*.xml` from the converted output. `TestXMLGeneration`
also validates each generated XML against the TicketBAI XSD on every
run, regardless of `--update`. Use `go test .` rather than
`go test ./...` because `--update` is only defined for the root and
`convert` test binaries.

XSD validation is pure Go via [`lestrrat-go/helium`][helium] — no
cgo, no system libraries. The wrapper schema in `test/schema/schema.xsd`
pre-imports a local copy of the W3C `xmldsig-core-schema.xsd` so no
network access is required at test time.

[helium]: https://github.com/lestrrat-go/helium

### Sending to the Bizkaia sandbox

`send-test.sh` at the repo root wraps `gobl-tbai send` with the Bizkaia
sandbox cert and the public Batuz test licence baked in:

```bash
./send-test.sh test/data/sample-invoice.json
```

It rebuilds `bin/gobl-tbai` on each invocation and POSTs to
`https://pruesarrerak.bizkaia.eus`. Any extra flags after the filename are
forwarded to the CLI (e.g. `--prev '{...}'` to chain).

The script expects:

- **Cert:** `test/certs/EntitateOrdezkaria_RepresentanteDeEntidad.p12`,
  password `IZDesa2025`. This is an Izenpe sandbox cert representing the
  fictitious entity `S7836107H`. Izenpe rotates these every ~4 years; when
  they expire (currently 2029) grab a fresh pack from the Izenpe
  "Garatzaileak / Desarrolladores" download page and update both the
  `.p12` and the `*_pin.txt` files in `test/certs/`.
- **Software identity:** the public Batuz test values
  (`A99800005` / `SOFTWARE GARANTE TICKETBAI PRUEBA`) paired with licence
  `TBAIBI00000000PRUEBA`. Batuz validates this combination on submission.
- **Supplier:** the invoice's supplier must use the cert's entity — that's
  why all bundled fixtures use `S7836107H` and the matching legal name. A
  mismatch triggers `N3_0000001` (cert not trusted for the supplier) or
  `N3_0000002` (legal-name mismatch).

Common rejection codes when iterating:

| Code | Meaning | Where to look |
|------|---------|---------------|
| `N3_0000001` | Signature not trusted for the supplier NIF | Cert expired, or the cert's `organizationIdentifier` doesn't match the invoice's `supplier.tax_id.code` |
| `N3_0000002` | Supplier legal name doesn't match the NIF | Update `supplier.name` to whatever Batuz returns in the error |
| `B4_2000013` | `NIF-IVA tiene un formato erróneo` | EU customer VATs are validated live against VIES; the bundled `invoice-es-nl-tbai-exempt.json` uses a placeholder NL VAT (`000099995B57`) that fails VIES and is not submittable — substitute a known-valid VAT from your own test data if you need to round-trip this fixture through the sandbox. |
| `B4_2000026` | `Las Claves indicadas no son compatibles` | The Bizkaia gateway rejects regime codes `19`, `51`, `52` and `53` — even though the TicketBAI XSD allows them and the Gipuzkoa/Araba gateways accept them. This is why `sample-invoice.json` (which legitimately produces `51` via its `general+eqs` rate, and is our regression case for the surcharge bug) won't round-trip through Bizkaia; use one of the `general`-rate fixtures (e.g. `sample-invoice2.json`) for submission tests. |
| `pkcs12: decryption password incorrect` | Wrong cert password | Check `test/certs/*_pin.txt` for the current pin (it's `IZDesa2025` for the 2025–2029 pack) |

The Araba and Gipuzkoa sandboxes need their own cert + licence packs;
`send-test.sh` only covers Bizkaia. (In practice the Gipuzkoa sandbox
currently accepts the Bizkaia developer software identity, but don't
rely on it.)
