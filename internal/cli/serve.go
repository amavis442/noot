package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/amavis442/noot/internal/httpapi"
	"github.com/amavis442/noot/web"
	"github.com/spf13/cobra"
)

func cmdServe(opts *options) *cobra.Command {
	var addr string
	c := &cobra.Command{
		Use:   "serve",
		Short: "Serve a web UI to search, read and create notes (Ctrl+C to stop)",
		Long: `Starts a local web server for the selected vault: the web UI at / and a JSON
API under /api/ (search, notes/{ref}, POST notes). The index is kept up to
date while it runs, like noot watch.

On a loopback address (the default) only requests for localhost are served.`,
		Example: `  noot serve
  noot -A serve --addr 127.0.0.1:8080`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, opts, true, func(e *env) error {
				ln, err := net.Listen("tcp", addr)
				if err != nil {
					return err
				}
				h := httpapi.New(e.svc, web.Dist())
				if tcp, ok := ln.Addr().(*net.TCPAddr); ok && tcp.IP.IsLoopback() {
					h = httpapi.LoopbackOnly(h)
				}
				srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}

				ctx, stop := signal.NotifyContext(e.ctx, os.Interrupt)
				defer stop()
				logf := func(f string, a ...any) {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s  %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(f, a...))
				}
				go func() {
					if err := e.svc.Watch(ctx, 300*time.Millisecond, logf); err != nil {
						logf("watch stopped: %v", err)
					}
				}()
				go func() {
					<-ctx.Done()
					shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					_ = srv.Shutdown(shutdown)
				}()

				fmt.Fprintf(e.out, "serving %s on http://%s\n", e.svc.Vault.Root, ln.Addr())
				if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
					return err
				}
				return nil
			})
		},
	}
	c.Flags().StringVar(&addr, "addr", "127.0.0.1:4242", "address to listen on")
	return c
}
