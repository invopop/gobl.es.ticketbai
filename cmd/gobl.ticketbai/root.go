package main

import (
	"io"
	"os"

	ticketbai "github.com/invopop/gobl.es.ticketbai"
	"github.com/invopop/gobl.es.ticketbai/internal/gateways"
	"github.com/invopop/gobl/l10n"
	_ "github.com/joho/godotenv/autoload"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type rootOpts struct {
	cert          string
	password      string
	swNIF         string
	swCompanyName string
	swName        string
	swVersion     string
	swLicense     string
	production    bool
}

func root() *rootOpts {
	return &rootOpts{}
}

func (o *rootOpts) cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "gobl.ticketbai",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(versionCmd())
	cmd.AddCommand(send(o).cmd())
	cmd.AddCommand(convert(o).cmd())
	cmd.AddCommand(cancel(o).cmd())

	return cmd
}

func (o *rootOpts) prepareFlags(f *pflag.FlagSet) {
	f.StringVar(&o.cert, "cert", os.Getenv("CERTIFICATE_PATH"), "Certificate for authentication")
	f.StringVar(&o.password, "password", os.Getenv("CERTIFICATE_PASSWORD"), "Password of the certificate")
	f.StringVar(&o.swNIF, "sw-nif", os.Getenv("SOFTWARE_COMPANY_NIF"), "NIF of the software company")
	f.StringVar(&o.swCompanyName, "sw-company-name", os.Getenv("SOFTWARE_COMPANY_NAME"), "Name of the software company")
	f.StringVar(&o.swName, "sw-name", os.Getenv("SOFTWARE_NAME"), "Name of the software")
	f.StringVar(&o.swVersion, "sw-version", os.Getenv("SOFTWARE_VERSION"), "Version of the software")
	f.StringVar(&o.swLicense, "sw-license", os.Getenv("SOFTWARE_LICENSE"), "License of the software")
	f.BoolVarP(&o.production, "production", "p", false, "Production environment")
}

func (o *rootOpts) software(zone l10n.Code) *ticketbai.Software {
	env := gateways.EnvironmentSandbox
	if o.production {
		env = gateways.EnvironmentProduction
	}
	return &ticketbai.Software{
		NIF:         o.swNIF,
		Name:        o.swName,
		CompanyName: o.swCompanyName,
		Version:     o.swVersion,
		Licenses: ticketbai.Licenses{
			env: {
				zone: o.swLicense,
			},
		},
	}
}

func (o *rootOpts) outputFilename(args []string) string {
	if len(args) >= 2 && args[1] != "-" {
		return args[1]
	}
	return ""
}

func openInput(cmd *cobra.Command, args []string) (io.ReadCloser, error) {
	if inFile := inputFilename(args); inFile != "" {
		return os.Open(inFile)
	}
	return io.NopCloser(cmd.InOrStdin()), nil
}

func (o *rootOpts) openOutput(cmd *cobra.Command, args []string) (io.WriteCloser, error) {
	if outFile := o.outputFilename(args); outFile != "" {
		flags := os.O_CREATE | os.O_WRONLY
		return os.OpenFile(outFile, flags, os.ModePerm)
	}
	return writeCloser{cmd.OutOrStdout()}, nil
}

type writeCloser struct {
	io.Writer
}

func (writeCloser) Close() error { return nil }
