package jsonapi

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/almighty/almighty-core/models"
	"github.com/goadesign/goa"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

const (
	// Media type for errors returned by the JSON API error handler middleware
	ErrorMediaIdentifier = "application/vnd.api+json"
)

func shortID() string {
	b := make([]byte, 6)
	io.ReadFull(rand.Reader, b)
	return base64.StdEncoding.EncodeToString(b)
}

// ErrorHandler turns a Go error into an JSONAPI HTTP response. It should be placed in the middleware chain
// below the logger middleware so the logger properly logs the HTTP response. ErrorHandler
// understands instances of goa.ServiceError and returns the status and response body embodied in
// them, it turns other Go error types into a 500 internal error response.
// If verbose is false the details of internal errors is not included in HTTP responses.
// If you use github.com/pkg/errors then wrapping the error will allow a trace to be printed to the logs
func ErrorHandler(service *goa.Service, verbose bool) goa.Middleware {
	return func(h goa.Handler) goa.Handler {
		return func(ctx context.Context, rw http.ResponseWriter, req *http.Request) error {
			e := h(ctx, rw, req)
			if e == nil {
				return nil
			}
			cause := errors.Cause(e)
			status := http.StatusInternalServerError
			var respBody interface{}
			respBody, status = ErrorToJSONAPIErrors(e)
			rw.Header().Set("Content-Type", ErrorMediaIdentifier)
			if err, ok := cause.(goa.ServiceError); ok {
				status = err.ResponseStatus()
				//respBody = err
				goa.ContextResponse(ctx).ErrorCode = err.Token()
				//rw.Header().Set("Content-Type", ErrorMediaIdentifier)
			} else {
				//respBody = e.Error()
				//rw.Header().Set("Content-Type", "text/plain")
			}
			if status >= 500 && status < 600 {
				//reqID := ctx.Value(reqIDKey)
				reqID := ctx.Value(1) // TODO remove this hack
				if reqID == nil {
					reqID = shortID()
					//ctx = context.WithValue(ctx, reqIDKey, reqID)
					ctx = context.WithValue(ctx, 1, reqID) // TODO remove this hack
				}
				goa.LogError(ctx, "uncaught error", "err", fmt.Sprintf("%+v", e), "id", reqID, "msg", respBody)
				if !verbose {
					rw.Header().Set("Content-Type", goa.ErrorMediaIdentifier)
					msg := models.NewInternalError(fmt.Sprintf("%s [%s]", http.StatusText(http.StatusInternalServerError), reqID))
					//respBody = goa.ErrInternal(msg)
					respBody, status = ErrorToJSONAPIErrors(msg)
					// Preserve the ID of the original error as that's what gets logged, the client
					// received error ID must match the original
					// TODO for JSONAPI this won't work I guess.
					if origErrID := goa.ContextResponse(ctx).ErrorCode; origErrID != "" {
						respBody.(*goa.ErrorResponse).ID = origErrID
					}
				}
			}
			return service.Send(ctx, status, respBody)
		}
	}
}