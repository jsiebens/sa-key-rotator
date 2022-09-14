package cmd

import (
	"encoding/json"
	"github.com/jsiebens/sa-key-rotator/pkg/sakeyrotator"
	"github.com/muesli/coral"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

func serverCommand() *coral.Command {
	command := &coral.Command{
		Use:          "server",
		SilenceUsage: true,
	}

	command.RunE = func(cmd *coral.Command, args []string) error {
		ctx := cmd.Context()

		logger := sakeyrotator.NewLogger(logLevel, stdout, stderr)
		rotator, err := sakeyrotator.NewRotator(ctx, logger)
		if err != nil {
			return err
		}

		http.HandleFunc("/", NewHandler(rotator, logger))

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

func NewHandler(rotator *sakeyrotator.Rotator, logger *sakeyrotator.Logger) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var messages []Message

		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("error reading request-body", "err", err)
			http.Error(w, "Bad Request (body)", http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(body, &messages); err != nil {
			logger.Error("error reading request-body", "err", err)
			http.Error(w, "Bad Request (body)", http.StatusBadRequest)
			return
		}

		var ok = true
		var wg sync.WaitGroup

		wg.Add(len(messages))

		errChannel := make(chan bool, 1)
		finished := make(chan bool, 1)

		for _, m := range messages {
			go func(x Message) {
				defer wg.Done()
				var valid = true

				if strings.TrimSpace(m.ServiceAccountEmail) == "" {
					logger.Warn("invalid request, service_account field is missing")
					valid = false
				}
				if strings.TrimSpace(m.BucketName) == "" {
					logger.Warn("invalid request, bucket field is missing")
					valid = false
				}
				if m.Days < 2 {
					logger.Warn("invalid request, days cannot be smaller than 2")
					valid = false
				}
				if m.RenewalWindow < 1 {
					logger.Warn("invalid request, renewal_window cannot be smaller than 1")
					valid = false
				}
				if m.RenewalWindow >= m.Days {
					logger.Warn("invalid request, renewal_window should be smaller than days")
					valid = false
				}

				if !valid {
					errChannel <- valid
					return
				}

				if err := rotator.Rotate(r.Context(), m.ServiceAccountEmail, sakeyrotator.DefaultName, m.BucketName, m.Days, m.RenewalWindow, false, false); err != nil {
					logger.Error("error rotating service account key",
						"service_account", m.ServiceAccountEmail,
						"err", err,
					)
					errChannel <- false
				}
			}(m)
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
