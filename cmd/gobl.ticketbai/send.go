// Package main provides the command line interface to the TicketBAI package.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/invopop/gobl"
	ticketbai "github.com/invopop/gobl.es.ticketbai"
	convert1 "github.com/invopop/gobl.es.ticketbai/convert"
	"github.com/invopop/xmldsig"
	"github.com/spf13/cobra"
)

type sendOpts struct {
	*rootOpts

	previous string
}

func send(o *rootOpts) *sendOpts {
	return &sendOpts{rootOpts: o}
}

func (c *sendOpts) cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send [infile]",
		Short: "Sends the GOBL invoice to the TicketBAI service",
		Run: func(cmd *cobra.Command, args []string) {
			handleError(c.runE(cmd, args))
		},
	}

	f := cmd.Flags()
	c.prepareFlags(f)

	f.StringVar(&c.previous, "prev", "", "Previous document fingerprint to chain with")

	return cmd
}

func (c *sendOpts) runE(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("unable to determine zone")
	}

	cert, err := xmldsig.LoadCertificate(c.cert, c.password)
	if err != nil {
		return err
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
		return err
	}

	td, err := tc.Convert(env)
	if err != nil {
		return err
	}

	var prev *convert1.ChainData
	if c.previous != "" {
		prev = new(convert1.ChainData)
		if err := json.Unmarshal([]byte(c.previous), prev); err != nil {
			return err
		}
	}

	err = tc.Fingerprint(td, prev)
	if err != nil {
		return err
	}

	if err := tc.Sign(td, env); err != nil {
		return err
	}

	err = tc.Post(cmd.Context(), env, td)
	if err != nil {
		return err
	}

	data, err := json.Marshal(td.ChainData())
	if err != nil {
		return err
	}
	fmt.Printf("Generated document with fingerprint: \n%s\n", string(data))

	return nil
}
