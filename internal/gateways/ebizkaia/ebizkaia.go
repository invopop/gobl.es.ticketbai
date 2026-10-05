// Package ebizkaia provides a gatewy for generating and sending documents to the
// Bizkaia region.
package ebizkaia

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"

	"github.com/invopop/gobl.es.ticketbai/convert"
)

// Bizkaia has extra complications when sending documents, so we define all the additional
// XML wrappers here.

// Constants used in headers
const (
	concepto                    = "LROE"
	apartado1                   = "1"
	apartado1_1                 = "1.1"
	Modelo240                   = "240"
	Modelo140                   = "140"
	schemaLROE240ConSGAlta      = "https://www.batuz.eus/fitxategiak/batuz/LROE/esquemas/LROE_PJ_240_1_1_FacturasEmitidas_ConSG_AltaPeticion_V1_0_2.xsd"
	schemaLROE240ConSGConsulta  = "https://www.batuz.eus/fitxategiak/batuz/LROE/esquemas/LROE_PJ_240_1_1_FacturasEmitidas_ConSG_ConsultaPeticion_V1_0_0.xsd"
	schemaLROE240ConSGAnulacion = "https://www.batuz.eus/fitxategiak/batuz/LROE/esquemas/LROE_PJ_240_1_1_FacturasEmitidas_ConSG_AnulacionPeticion_V1_0_0.xsd"
	schemaLROE140ConSGAlta      = "https://www.batuz.eus/fitxategiak/batuz/LROE/esquemas/LROE_PF_140_1_1_Ingresos_ConfacturaConSG_AltaPeticion_V1_0_2.xsd"
	schemaLROE140ConSGConsulta  = "https://www.batuz.eus/fitxategiak/batuz/LROE/esquemas/LROE_PF_140_1_1_Ingresos_ConfacturaConSG_ConsultaPeticion_V1_0_0.xsd"
	schemaLROE140ConSGAnulacion = "https://www.batuz.eus/fitxategiak/batuz/LROE/esquemas/LROE_PF_140_1_1_Ingresos_ConfacturaConSG_AnulacionPeticion_V1_0_0.xsd"
)

const (
	operacionEnumAlta                           = "A00"
	operacionEnumAltaDevolucionViajeros         = "A01"
	operacionEnumModificacion                   = "M00"
	operacionEnumModificacionDevolucionViajeros = "M01"
	operacionEnumAnulacion                      = "AN0"
	operacionEnumConsulta                       = "C00"
)

// Request tries to keep all the request information in one place ready to send to the
// Bizkaia servers. They really out did themselves while defining these connections making it
// increadibly complex.
type Request struct {
	Header  []byte // JSON special data header
	Payload []byte // already gzipped
}

// N3Header is the structure that needs to be included in requests containing
// details about what is being sent. For some reason, they decided instead of defining
// different URLs according to the use-case, to just have a single end point with a JSON
// object included in the HTTP headers. Bizarre.
type N3Header struct {
	Concepto        string            `json:"con"` // always "LROE"
	Apartado        string            `json:"apa"`
	Interesado      N3Interesado      `json:"inte"`
	DatosRelevantes N3DatosRelevantes `json:"drs"`
}

// N3Interesado defines fields that describe the taxable entity.
type N3Interesado struct {
	NIF       string `json:"nif"`
	Nombre    string `json:"nrs"`
	Apellido1 string `json:"ap1,omitempty"`
	Apellido2 string `json:"ap2,omitempty"`
}

// N3DatosRelevantes defines the struct used to store additional
// information about the request, used in the header.
type N3DatosRelevantes struct {
	Modelo    string `json:"mode"`
	Ejercicio string `json:"ejer"` // invoice issue year
}

// CabeceraType contains the operation headers.
type CabeceraType struct {
	Modelo             string
	Capitulo           string // nolint:misspell
	Subcapitulo        string `xml:",omitempty"`
	Operacion          string
	Version            string
	Ejercicio          string
	ObligadoTributario *NIFPersonaType
}

// LROEPJ240FacturasEmitidasConSGAltaPeticion is used for uploading invoices. "240" refers to
// the type of entity which in this case is a "business", and "ConSG" means the request is
// from a "software garante", i.e. Invopop.
type LROEPJ240FacturasEmitidasConSGAltaPeticion struct {
	XMLName       xml.Name `xml:"lrpjfecsgap:LROEPJ240FacturasEmitidasConSGAltaPeticion"`
	LROENamespace string   `xml:"xmlns:lrpjfecsgap,attr"`

	Cabecera         *CabeceraType
	FacturasEmitidas *FacturasEmitidasConSGCodificadoType
}

// LROEPJ240FacturasEmitidasConSGAltaRespuesta represents the response from the server
// when uploading invoices.
type LROEPJ240FacturasEmitidasConSGAltaRespuesta struct {
	Registros *RegistrosFacturaConSGType
}

// RegistrosFacturaConSGType contains the response for all invoices proccessed in a upload request.
type RegistrosFacturaConSGType struct {
	Registro []*RegistroFacturaConSGType
}

// RegistroFacturaConSGType contains the response for a single invoice proccessed in a upload
// request.
type RegistroFacturaConSGType struct {
	SituacionRegistro *SituacionRegistroType
}

// SituacionRegistroType details about the outcome of uploading a single invoice.
type SituacionRegistroType struct {
	CodigoErrorRegistro        string
	DescripcionErrorRegistroES string
}

// LROEPJ240FacturasEmitidasConSGConsultaPeticion represents a request to fetch invoices.
type LROEPJ240FacturasEmitidasConSGConsultaPeticion struct {
	XMLName       xml.Name `xml:"lrpjfecsgcp:LROEPJ240FacturasEmitidasConSGConsultaPeticion"`
	LROENamespace string   `xml:"xmlns:lrpjfecsgcp,attr"`

	Cabecera                            *CabeceraType
	FiltroConsultaFacturasEmitidasConSG *FiltroConsultaFacturasEmitidasType
}

// FiltroConsultaFacturasEmitidasType contains the details of an invoice query.
type FiltroConsultaFacturasEmitidasType struct {
	CabeceraFactura   *CabeceraFacturaConsultaType
	NumPaginaConsulta int
}

// CabeceraFacturaConsultaType contains the header of an invoice query.
type CabeceraFacturaConsultaType struct {
	SerieFactura           string `xml:",omitempty"`
	NumFactura             string `xml:",omitempty"`
	FechaExpedicionFactura *FechaDesdeHastaType
}

// FechaDesdeHastaType represants a date range
type FechaDesdeHastaType struct {
	Desde string `xml:",omitempty"`
	Hasta string `xml:",omitempty"`
}

// LROEPJ240FacturasEmitidasConSGConsultaRespuesta represents the response from the server
// when fetching invoices.
type LROEPJ240FacturasEmitidasConSGConsultaRespuesta struct {
	FacturasEmitidas *FacturasEmitidasConSGConsultaRespuestaType
}

// FacturasEmitidasConSGConsultaRespuestaType contains the response for all invoices fetched.
type FacturasEmitidasConSGConsultaRespuestaType struct {
	FacturaEmitida []*FacturaEmitidaConSGConsultaRespuestaType
}

// FacturaEmitidaConSGConsultaRespuestaType contains the response for a single invoice fetched.
type FacturaEmitidaConSGConsultaRespuestaType struct {
	TicketBai *TicketBaiType
}

// TicketBaiType contains the details of a fetched invoice.
type TicketBaiType struct {
	Cabecera   *convert.Cabecera
	Sujetos    *convert.Sujetos
	Factura    *convert.Factura
	HuellaTBAI *convert.HuellaTBAI
	Signature  string
}

// LROEPJ240FacturasEmitidasConSGAnulacionPeticion is used for cancelling invoices.
type LROEPJ240FacturasEmitidasConSGAnulacionPeticion struct {
	XMLName       xml.Name `xml:"lrpjfecsgap:LROEPJ240FacturasEmitidasConSGAnulacionPeticion"`
	LROENamespace string   `xml:"xmlns:lrpjfecsgap,attr"`

	Cabecera         *CabeceraType
	FacturasEmitidas *AnulacionesFacturasEmitidasConSGType
}

// FacturasEmitidasConSGCodificadoType holds an array of invoices
// to send.
type FacturasEmitidasConSGCodificadoType struct {
	FacturaEmitida []*DetalleEmitidaConSGCodificadoType // max length 1000
}

// AnulacionesFacturasEmitidasConSGType holds an array of invoices to cancel.
type AnulacionesFacturasEmitidasConSGType struct {
	FacturaEmitida []*AnulacionFacturaConSGType
}

// DetalleEmitidaConSGCodificadoType contains the invoice to upload
type DetalleEmitidaConSGCodificadoType struct {
	TicketBai string // base64 data
}

// AnulacionFacturaConSGType contains the invoice to cancel
type AnulacionFacturaConSGType struct {
	AnulacionTicketBai string // base64 data
}

// LROEPF140IngresosConFacturaConSGAltaPeticion is used by individuals (persona física)
// for uploading invoices under Modelo 140.
type LROEPF140IngresosConFacturaConSGAltaPeticion struct {
	XMLName       xml.Name `xml:"lrpficfcsgap:LROEPF140IngresosConFacturaConSGAltaPeticion"`
	LROENamespace string   `xml:"xmlns:lrpficfcsgap,attr"`

	Cabecera *CabeceraType
	Ingresos *IngresosConSGCodificadoType
}

// LROEPF140IngresosConFacturaConSGAltaRespuesta represents the response from the server
// when uploading invoices under Modelo 140.
type LROEPF140IngresosConFacturaConSGAltaRespuesta struct {
	Registros *RegistrosFacturaConSGType
}

// LROEPF140IngresosConFacturaConSGConsultaPeticion represents a request to fetch invoices
// under Modelo 140.
type LROEPF140IngresosConFacturaConSGConsultaPeticion struct {
	XMLName       xml.Name `xml:"lrpficfcsgcp:LROEPF140IngresosConFacturaConSGConsultaPeticion"`
	LROENamespace string   `xml:"xmlns:lrpficfcsgcp,attr"`

	Cabecera                    *CabeceraType
	FiltroConsultaIngresosConSG *FiltroConsultaFacturasEmitidasType
}

// LROEPF140IngresosConFacturaConSGConsultaRespuesta represents the response from the server
// when fetching invoices under Modelo 140.
type LROEPF140IngresosConFacturaConSGConsultaRespuesta struct {
	FacturasEmitidas *FacturasEmitidasConSGConsultaRespuestaType
}

// LROEPF140IngresosConFacturaConSGAnulacionPeticion is used by individuals for cancelling
// invoices under Modelo 140.
type LROEPF140IngresosConFacturaConSGAnulacionPeticion struct {
	XMLName       xml.Name `xml:"lrpficfcsgap:LROEPF140IngresosConFacturaConSGAnulacionPeticion"`
	LROENamespace string   `xml:"xmlns:lrpficfcsgap,attr"`

	Cabecera *CabeceraType
	Ingresos *AnulacionesIngresosConSGType
}

// IngresosConSGCodificadoType holds an array of income records to send under Modelo 140.
type IngresosConSGCodificadoType struct {
	Ingreso []*IngresoConSGCodificadoType // max length 1000
}

// IngresoConSGCodificadoType contains a single income record under Modelo 140.
type IngresoConSGCodificadoType struct {
	TicketBai string // base64 data
	Renta     *RentaIngresosType
}

// RentaIngresosType wraps the income detail breakdown for Modelo 140.
type RentaIngresosType struct {
	DetalleRenta []*DetalleRentaIngresosType // 1..10
}

// DetalleRentaIngresosType describes a single income detail entry. Only Epigrafe is
// populated by the converter today; the remaining XSD fields are exposed for follow-up
// wiring through GOBL extensions.
type DetalleRentaIngresosType struct {
	TerritorioAltaActividad                  string `xml:",omitempty"`
	Epigrafe                                 string
	NumeroFijoOReferenciaCatastral           string `xml:",omitempty"`
	IngresoAComputarIRPFDiferenteBaseImpoIVA string `xml:",omitempty"`
	CausaIngresoIRPFDiferenteBaseImpoIVA     string `xml:",omitempty"`
	ImporteIngresoIRPF                       string `xml:",omitempty"`
	CriterioCobrosYPagos                     string `xml:",omitempty"`
}

// AnulacionesIngresosConSGType holds an array of income records to cancel under Modelo 140.
type AnulacionesIngresosConSGType struct {
	Ingreso []*AnulacionIngresoConSGType
}

// AnulacionIngresoConSGType contains the income record to cancel under Modelo 140.
// Cancellations carry only the TicketBai reference — no Renta block.
type AnulacionIngresoConSGType struct {
	AnulacionTicketBai string // base64 data
}

// NIFPersonaType contains the identification details of a taxable natural or
// legal person.
type NIFPersonaType struct {
	NIF                        string
	ApellidosNombreRazonSocial string // max 120
}

// Supplier contains the details of the supplier who is making the
// request.
type Supplier struct {
	Year     string // invoice issue year
	NIF      string // Tax code
	Name     string // Name of the company
	Model    string // Modelo140 or Modelo240; empty defaults to Modelo240
	Activity string // IAE Epigrafe; only used when Model == Modelo140
}

// NewCreateRequest assembles a new Create request
func NewCreateRequest(sup *Supplier, payload []byte) (*Request, error) {
	if sup.Model == Modelo140 {
		body := &LROEPF140IngresosConFacturaConSGAltaPeticion{
			LROENamespace: schemaLROE140ConSGAlta,
			Cabecera:      newCabeceraType(sup, operacionEnumAlta),
			Ingresos: &IngresosConSGCodificadoType{
				Ingreso: []*IngresoConSGCodificadoType{
					{
						TicketBai: base64.StdEncoding.EncodeToString(payload),
						Renta: &RentaIngresosType{
							DetalleRenta: []*DetalleRentaIngresosType{
								{Epigrafe: sup.Activity},
							},
						},
					},
				},
			},
		}
		return newRequest(sup, body)
	}

	body := &LROEPJ240FacturasEmitidasConSGAltaPeticion{
		LROENamespace: schemaLROE240ConSGAlta,
		Cabecera:      newCabeceraType(sup, operacionEnumAlta),
		FacturasEmitidas: &FacturasEmitidasConSGCodificadoType{
			FacturaEmitida: []*DetalleEmitidaConSGCodificadoType{
				{
					TicketBai: base64.StdEncoding.EncodeToString(payload),
				},
			},
		},
	}

	return newRequest(sup, body)
}

// NewFetchRequest assembles a new Fetch request
func NewFetchRequest(sup *Supplier, page int, head *convert.CabeceraFactura) (*Request, error) {
	var cabeceraFiltro *CabeceraFacturaConsultaType
	if head != nil {
		cabeceraFiltro = &CabeceraFacturaConsultaType{
			NumFactura:   head.NumFactura,
			SerieFactura: head.SerieFactura,
			FechaExpedicionFactura: &FechaDesdeHastaType{
				Desde: head.FechaExpedicionFactura,
				Hasta: head.FechaExpedicionFactura,
			},
		}
	}

	if sup.Model == Modelo140 {
		body := &LROEPF140IngresosConFacturaConSGConsultaPeticion{
			LROENamespace: schemaLROE140ConSGConsulta,
			Cabecera:      newCabeceraType(sup, operacionEnumConsulta),
			FiltroConsultaIngresosConSG: &FiltroConsultaFacturasEmitidasType{
				CabeceraFactura:   cabeceraFiltro,
				NumPaginaConsulta: page,
			},
		}
		return newRequest(sup, body)
	}

	body := &LROEPJ240FacturasEmitidasConSGConsultaPeticion{
		LROENamespace: schemaLROE240ConSGConsulta,
		Cabecera:      newCabeceraType(sup, operacionEnumConsulta),
		FiltroConsultaFacturasEmitidasConSG: &FiltroConsultaFacturasEmitidasType{
			CabeceraFactura:   cabeceraFiltro,
			NumPaginaConsulta: page,
		},
	}

	return newRequest(sup, body)
}

// NewCancelRequest assembles a new Cancel request
func NewCancelRequest(sup *Supplier, payload []byte) (*Request, error) {
	if sup.Model == Modelo140 {
		body := &LROEPF140IngresosConFacturaConSGAnulacionPeticion{
			LROENamespace: schemaLROE140ConSGAnulacion,
			Cabecera:      newCabeceraType(sup, operacionEnumAnulacion),
			Ingresos: &AnulacionesIngresosConSGType{
				Ingreso: []*AnulacionIngresoConSGType{
					{
						AnulacionTicketBai: base64.StdEncoding.EncodeToString(payload),
					},
				},
			},
		}
		return newRequest(sup, body)
	}

	body := &LROEPJ240FacturasEmitidasConSGAnulacionPeticion{
		LROENamespace: schemaLROE240ConSGAnulacion,
		Cabecera:      newCabeceraType(sup, operacionEnumAnulacion),
		FacturasEmitidas: &AnulacionesFacturasEmitidasConSGType{
			FacturaEmitida: []*AnulacionFacturaConSGType{
				{
					AnulacionTicketBai: base64.StdEncoding.EncodeToString(payload),
				},
			},
		},
	}

	return newRequest(sup, body)
}

func newCabeceraType(sup *Supplier, op string) *CabeceraType {
	model := sup.Model
	if model == "" {
		model = Modelo240
	}
	head := &CabeceraType{
		Modelo:      model,
		Capitulo:    apartado1, // nolint:misspell
		Subcapitulo: apartado1_1,
		Operacion:   op,
		Version:     "1.0",
		Ejercicio:   sup.Year,
		ObligadoTributario: &NIFPersonaType{
			NIF:                        sup.NIF,
			ApellidosNombreRazonSocial: sup.Name,
		},
	}
	return head
}

func newRequest(sup *Supplier, body any) (*Request, error) {
	req := new(Request)

	bdata, err := xml.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode body to XML: %w", err)
	}
	req.Payload, err = compressBody(bdata)
	if err != nil {
		return nil, fmt.Errorf("compressing body: %w", err)
	}

	jhead := newN3Header(sup)
	req.Header, err = json.Marshal(jhead)
	if err != nil {
		return nil, fmt.Errorf("json header: %w", err)
	}

	return req, nil
}

func newN3Header(sup *Supplier) *N3Header {
	model := sup.Model
	if model == "" {
		model = Modelo240
	}
	// prepare the request data header
	head := &N3Header{
		Concepto: concepto,
		Apartado: apartado1_1, // for invoices
		Interesado: N3Interesado{
			NIF:    sup.NIF,
			Nombre: sup.Name,
		},
		DatosRelevantes: N3DatosRelevantes{
			Modelo:    model,
			Ejercicio: sup.Year,
		},
	}
	return head
}

func compressBody(data []byte) ([]byte, error) {
	// Gzip the data
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	if _, err := zw.Write(data); err != nil {
		return nil, fmt.Errorf("compressing: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("closing gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// FirstErrorCode returns the first error code in the response.
func (r *LROEPJ240FacturasEmitidasConSGAltaRespuesta) FirstErrorCode() string {
	if r.Registros == nil || len(r.Registros.Registro) == 0 {
		return ""
	}

	return r.Registros.Registro[0].SituacionRegistro.CodigoErrorRegistro
}

// FirstErrorDescription returns the first error description in the response.
func (r *LROEPJ240FacturasEmitidasConSGAltaRespuesta) FirstErrorDescription() string {
	if r.Registros == nil || len(r.Registros.Registro) == 0 {
		return ""
	}

	return r.Registros.Registro[0].SituacionRegistro.DescripcionErrorRegistroES
}

// FirstErrorCode returns the first error code in the response.
func (r *LROEPF140IngresosConFacturaConSGAltaRespuesta) FirstErrorCode() string {
	if r.Registros == nil || len(r.Registros.Registro) == 0 {
		return ""
	}

	return r.Registros.Registro[0].SituacionRegistro.CodigoErrorRegistro
}

// FirstErrorDescription returns the first error description in the response.
func (r *LROEPF140IngresosConFacturaConSGAltaRespuesta) FirstErrorDescription() string {
	if r.Registros == nil || len(r.Registros.Registro) == 0 {
		return ""
	}

	return r.Registros.Registro[0].SituacionRegistro.DescripcionErrorRegistroES
}
