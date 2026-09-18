package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/sinanganiz/commitography/internal/server"
)

func newServeCommand(env environment) *cobra.Command {
	var options server.Options

	cmd := &cobra.Command{
		Use:           "serve",
		Short:         "Start the local web dashboard",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, serveOptions, logger, err := composeServe(env, options)
			if err != nil {
				return err
			}
			for _, root := range app.EmptyAllowedRoots() {
				logger.Warn("allowed root %q is empty; if it is a mount, check that its source path exists", root)
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			err = server.Serve(ctx, serveOptions)
			// The manager owns every analysis goroutine, and none outlives the
			// command (ADR-0044 clause 1).
			app.Jobs.CancelAll()
			app.Jobs.Wait()
			return err
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&options.ListenAddress, "listen", "127.0.0.1:8080", "HTTP listen address")
	flags.BoolVar(&options.Open, "open", false, "Open the dashboard in the default browser")
	flags.StringArrayVar(&options.AllowedRoots, "allowed-root", nil, "Allowed repository root; repeatable")
	return cmd
}
