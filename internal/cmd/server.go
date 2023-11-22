package cmd

import (
	"encoding/json"
	"github.com/jsiebens/sa-key-rotator/pkg/sakeyrotator"
	"github.com/spf13/cobra"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
)

func serverCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "server",
		SilenceUsage: true,
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		rotator, err := sakeyrotator.NewRotator(ctx)
		if err != nil {
			return err
		}

		http.HandleFunc("/", NewHandler(rotator))

		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			return err
		}

		return nil
	}

	return command
}

func NewHandler(rotator *sakeyrotator.Rotator) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var messages []Message

		body, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("error reading request-body", "err", err)
			http.Error(w, "Bad Request (body)", http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(body, &messages); err != nil {
			slog.Error("error reading request-body", "err", err)
			http.Error(w, "Bad Request (body)", http.StatusBadRequest)
			return
		}

		var ok = true
		var wg sync.WaitGroup

		wg.Add(len(messages))

		errChannel := make(chan bool, 1)
		finished := make(chan bool, 1)

		for _, message := range messages {
			go func(x Message) {
				defer wg.Done()
				var valid = true

				if strings.TrimSpace(x.ServiceAccountEmail) == "" {
					slog.Warn("invalid request, service_account field is missing")
					valid = false
				}
				if strings.TrimSpace(x.BucketName) == "" {
					slog.Warn("invalid request, bucket field is missing")
					valid = false
				}
				if x.Days < 2 {
					slog.Warn("invalid request, days cannot be smaller than 2")
					valid = false
				}
				if x.RenewalWindow < 1 {
					slog.Warn("invalid request, renewal_window cannot be smaller than 1")
					valid = false
				}
				if x.RenewalWindow >= x.Days {
					slog.Warn("invalid request, renewal_window should be smaller than days")
					valid = false
				}

				if !valid {
					errChannel <- valid
					return
				}

				if err := rotator.Rotate(r.Context(), x.ServiceAccountEmail, sakeyrotator.DefaultName, x.BucketName, x.Days, x.RenewalWindow, false, false); err != nil {
					slog.Error("error rotating service account key",
						"service_account", x.ServiceAccountEmail,
						"err", err,
					)
					errChannel <- false
				}
			}(message)
		}

		go func() {
			wg.Wait()
			close(finished)
		}()

		for {
			select {
			case <-finished:
				if !ok {
					http.Error(w, "Bad Request (body)", http.StatusBadRequest)
				}
				return
			case b := <-errChannel:
				ok = ok && b
			}
		}
	}
}

type Message struct {
	ServiceAccountEmail string `json:"service_account"`
	BucketName          string `json:"bucket"`
	Days                int    `json:"days"`
	RenewalWindow       int    `json:"renewal_window"`
}
