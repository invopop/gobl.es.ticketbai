// Package ca is used to provide the root certificates used by the TicketBAI services.
//
// Legacy Izenpe CAs, sourced from
// https://www.izenpe.eus/descarga-de-certificados/webize01-cndoctecnica/es/:
//
//   - RAIZ2007_cert_sha256.pem
//   - AAPPNR_cert_sha256.pem
//   - SSLEV_cert_signing_1_2018.pem
//
// Sectigo roots, sourced from the "CAs SSL Sectigo" tab at
// https://www.izenpe.eus/soporte-y-documentacion-tecnica/webize01-cndoctecnica/es/:
//
//   - SectigoPublicServerAuthenticationRootR46.pem
//   - SectigoPublicServerAuthenticationRootE46.pem
//
// The legacy Izenpe CAs can be removed once the production endpoints of the three
// councils have switched to the new Sectigo chain.
//
// Certificates were converted to PEM format with:
//
//	openssl x509 -in <cert>.crt -outform PEM -out <cert>.pem
package ca

import "embed"

//go:embed *.pem

// Content contains the root certificates used by the TicketBAI services.
var Content embed.FS
