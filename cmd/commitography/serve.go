package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/sinanganiz/commitography/internal/server"
)

func newServeCommand() *cobra.Command {
	var options server.Options
	app := server.NewApp(nil)
	options.Handler = app.Handler()
	options.OnShutdown = app.Jobs.CancelAll

	cmd := &cobra.Command{
		Use:           "serve",
		Short:         "Start the local web dashboard",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return server.Serve(ctx, options)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&options.ListenAddress, "listen", "127.0.0.1:8080", "HTTP listen address")
	flags.BoolVar(&options.Open, "open", false, "Open the dashboard in the default browser")
	flags.StringArrayVar(&options.AllowedRoots, "allowed-root", nil, "Allowed repository root; repeatable")
	return cmd
}
