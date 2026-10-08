// Package main provides the command line interface to the TicketBAI package.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/invopop/gobl"
	ticketbai "github.com/invopop/gobl.es.ticketbai"
	"github.com/invopop/xmldsig"
	"github.com/spf13/cobra"
)

type cancelOpts struct {
	*rootOpts
}

func cancel(o *rootOpts) *cancelOpts {
	return &cancelOpts{rootOpts: o}
}

func (c *cancelOpts) cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel [infile]",
		Short: "Cancels an invoice in the TicketBAI provider",
		RunE:  c.runE,
	}

	f := cmd.Flags()
	c.prepareFlags(f)

	return cmd
}

func (c *cancelOpts) runE(cmd *cobra.Command, args []string) error {
	input, err := openInput(cmd, args)
	if err != nil {
		return err
	}
	defer input.Close() // nolint:errcheck

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(input); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	env := new(gobl.Envelope)
	if err := json.Unmarshal(buf.Bytes(), env); err != nil {
		return fmt.Errorf("unmarshaling gobl envelope: %w", err)
	}
	zone := ticketbai.ZoneFor(env)
	if zone == "" {
		return fmt.Errorf("no zone found in envelope")
	}

	cert, err := xmldsig.LoadCertificate(c.cert, c.password)
	if err != nil {
		panic(err)
	}

	opts := []ticketbai.Option{
		ticketbai.WithCertificate(cert),
		ticketbai.WithThirdPartyIssuer(),
	}

	if c.production {
		opts = append(opts, ticketbai.InProduction())
	} else {
		opts = append(opts, ticketbai.InSandbox())
	}

	tc, err := ticketbai.New(c.software(zone), zone, opts...)
	if err != nil {
		panic(err)
	}

	tcd, err := tc.GenerateCancel(env)
	if err != nil {
		panic(err)
	}

	err = tc.FingerprintCancel(tcd)
	if err != nil {
		panic(err)
	}

	if err := tc.SignCancel(tcd, env); err != nil {
		panic(err)
	}

	err = tc.Cancel(cmd.Context(), env, tcd)
	if err != nil {
		panic(err)
	}

	return nil
}
