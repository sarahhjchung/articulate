package api

import (
	"articulate/api/oapigen"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

const apiVersion = "v1"

type API struct {
	ctrl   Server
	port   int
	server *http.Server
}

type Config struct {
	Port       int
	Controller Server
}

func NewAPI(ctx context.Context, conf Config) (*API, error) {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(withRequestID)

	api := &API{
		ctrl: conf.Controller,
		port: conf.Port,
	}

	r.Route(fmt.Sprintf("/%s", apiVersion), func(r chi.Router) {
		r.Mount(fmt.Sprintf("/%s", "prompts"),
			newPromptHandler(api.ctrl))

		r.Mount(fmt.Sprintf("/%s", "blobs"),
			newBlobHandler(api.ctrl))

		r.Mount(fmt.Sprintf("/%s", "health"),
			newHealthHandler(api.ctrl))
	})

	r.Group(func(r chi.Router) {
		server := Handlers{
			PromptHandler: NewPromptHandler(api.ctrl),
			BlobHandler:   NewBlobHandler(api.ctrl),
			HealthHandler: NewHealthHandler(api.ctrl),
		}

		oapigen.HandlerFromMux(server, r)
	})

	http.ListenAndServe(fmt.Sprintf(":%d", api.port), r)

	api.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", api.port),
		Handler: r,
	}

	return api, nil
}

func (api *API) Serve(ctx context.Context) error {
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if err := api.server.Shutdown(ctxShutDown); err != nil {
					log.Err(err).Msg("error stopping api server")
				} else {
					log.Info().Msg("shutdown")
				}
				return
			}
		}
	}()

	err := api.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	wg.Wait()
	return ctx.Err()
}