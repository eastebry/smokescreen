package cmd

import (
	"errors"
	"github.com/stripe/smokescreen/pkg/smokescreen"
	"gopkg.in/urfave/cli.v1"
	"net"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

// Process command line args into a configuration object.
// As a side-effect, processing the "--help" argument will cause the program to
// print the the help message and exit.  If args is nil, os.Args will be used.
// If logger is nil, a default logger will be created and included in the
// returned configuration.
func NewConfiguration(args []string, logger *log.Logger) (*smokescreen.Config, error) {
	if args == nil {
		args = os.Args
	}

	var configToReturn *smokescreen.Config

	app := cli.NewApp()
	app.Name = "smokescreen"
	app.Usage = "A simple HTTP proxy that prevents SSRF and can restrict destinations"
	app.ArgsUsage = " " // blank but non-empty to suppress default "[arguments...]"

	// Suppress "help" subcommand, as we have no other subcommands.
	// Unfortunately, this also suppresses "--help", so we'll add it back in
	// manually below.  See https://github.com/urfave/cli/issues/523
	app.HideHelp = true

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name: "help",
			Usage: "Show this help text.",
		},
		cli.StringFlag{
			Name:  "listen-ip",
			Usage: "listen on interface with address `IP`.\n\t\tThis argument is ignored when running under Einhorn. (default: any)",
		},
		cli.IntFlag{
			Name:  "listen-port",
			Value: 4750,
			Usage: "listen on port `PORT`.\n\t\tThis argument is ignored when running under Einhorn.",
		},
		cli.DurationFlag{
			Name:  "timeout",
			Value: time.Duration(10) * time.Second,
			Usage: "Time out after `DURATION` when connecting.",
		},
		cli.StringFlag{
			Name:  "maintenance-file",
			Usage: "Watch `FILE` for maintenance mode.\n\t\tHTTP(S) requests to /healthcheck return 404 if the file's permissions are set to 000.",
		},
		cli.BoolFlag{
			Name:  "proxy-protocol",
			Usage: "Enable PROXY protocol support.",
		},
		cli.StringSliceFlag{
			Name:  "deny-range",
			Usage: "Add `RANGE`(in CIDR notation) to list of blocked IP ranges.  Repeatable.",
		},
		cli.StringSliceFlag{
			Name:  "allow-range",
			Usage: "Add `RANGE` (in CIDR notation) to list of allowed IP ranges.  Repeatable.",
		},
		cli.StringFlag{
			Name:  "egress-acl-file",
			Usage: "Validate egress traffic against `FILE`",
		},
		cli.StringFlag{
			Name:  "statsd-address",
			Value: "127.0.0.1:8200",
			Usage: "Send metrics to statsd at `ADDRESS` (IP:port).",
		},
		cli.StringFlag{
			Name:  "tls-server-bundle-file",
			Usage: "Authenticate to clients using key and certs from `FILE`",
		},
		cli.StringSliceFlag{
			Name:  "tls-client-ca-file",
			Usage: "Validate client certificates using Certificate Authority from `FILE`",
		},
		cli.StringSliceFlag{
			Name:  "tls-crl-file",
			Usage: "Verify validity of client certificates against Certificate Revocation List from `FILE`",
		},
		cli.BoolFlag{
			Name:  "danger-allow-access-to-private-ranges",
			Usage: "WARNING: circumvent the check preventing client to reach hosts in private networks - It will make you vulnerable to SSRF.",
		},
		cli.StringFlag{
			Name:  "error-message-on-deny",
			Usage: "Display `MESSAGE` in the HTTP response if proxying request is denied",
		},
	}

	app.Action = func(c *cli.Context) error {
		if c.Bool("help") {
			cli.ShowAppHelpAndExit(c, 0)
		}
		if len(c.Args()) > 0 {
			return errors.New("Received unexpected non-option argument(s)")
		}

		var err error
		var cidrBlacklist []net.IPNet
		var cidrBlacklistExemptions []net.IPNet

		for _, cidrBlock := range smokescreen.PrivateNetworkStrings {
			cidrBlacklist, err = smokescreen.AddCidrToSlice(cidrBlacklist, cidrBlock)
			if err != nil {
				return err
			}
		}

		for _, cidrBlock := range c.StringSlice("deny-range") {
			cidrBlacklist, err = smokescreen.AddCidrToSlice(cidrBlacklist, cidrBlock)
			if err != nil {
				return err
			}
		}

		for _, cidrBlock := range c.StringSlice("allow-range") {
			cidrBlacklistExemptions, err = smokescreen.AddCidrToSlice(cidrBlacklistExemptions, cidrBlock)
			if err != nil {
				return err
			}
		}

		conf, err := smokescreen.NewConfig(
			logger,
			c.String("listen-ip"),
			c.Int("listen-port"),
			cidrBlacklist,
			cidrBlacklistExemptions,
			c.Duration("timeout"),
			60*time.Second,
			c.String("maintenance-file"),
			c.String("statsd-address"),
			c.String("egress-acl-file"),
			c.Bool("proxy-protocol"),
			c.String("tls-server-bundle-file"),
			c.StringSlice("tls-client-ca-file"),
			c.StringSlice("tls-crl-file"),
			c.Bool("danger-allow-access-to-private-ranges"),
			c.String("error-message-on-deny"),
		)
		if err != nil {
			return err
		}

		conf.StatsdClient.Namespace = "smokescreen."

		configToReturn = conf
		return nil
	}

	err := app.Run(args)

	return configToReturn, err
}