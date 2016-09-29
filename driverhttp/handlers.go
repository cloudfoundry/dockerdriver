package driverhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	cf_http_handlers "code.cloudfoundry.org/cfhttp/handlers"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/voldriver"
	"github.com/tedsuo/rata"
	"context"
)

// At present, Docker ignores HTTP status codes, and requires errors to be returned in the response body.  To
// comply with this API, we will return 200 in all cases
const (
	StatusInternalServerError = http.StatusOK
	StatusOK                  = http.StatusOK
)

func NewHandler(logger lager.Logger, client voldriver.Driver) (http.Handler, error) {
	logger = logger.Session("server")
	logger.Info("start")
	defer logger.Info("end")

	var handlers = rata.Handlers{
		voldriver.ActivateRoute:     newActivateHandler(logger, client),
		voldriver.GetRoute:          newGetHandler(logger, client),
		voldriver.ListRoute:         newListHandler(logger, client),
		voldriver.PathRoute:         newPathHandler(logger, client),
		voldriver.CreateRoute:       newCreateHandler(logger, client),
		voldriver.MountRoute:        newMountHandler(logger, client),
		voldriver.UnmountRoute:      newUnmountHandler(logger, client),
		voldriver.RemoveRoute:       newRemoveHandler(logger, client),
		voldriver.CapabilitiesRoute: newCapabilitiesHandler(logger, client),
	}

	return rata.NewRouter(voldriver.Routes, handlers)
}

func newActivateHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-activate")
		logger.Info("start")
		defer logger.Info("end")

		ctx, cancel := withCancel(logger, req.Context(), w)
		defer cancel()

		activateResponse := client.Activate(ctx)
		if activateResponse.Err != "" {
			logger.Error("failed-activating-driver", fmt.Errorf(activateResponse.Err))
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, activateResponse)
			return
		}

		logger.Debug("activate-response", lager.Data{"activation": activateResponse})
		cf_http_handlers.WriteJSONResponse(w, StatusOK, activateResponse)
	}
}

func newGetHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-get")
		logger.Info("start")
		defer logger.Info("end")

		ctx, cancel := withCancel(logger, req.Context(), w)
		defer cancel()

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logger.Error("failed-reading-get-request-body", err)
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, voldriver.MountResponse{Err: err.Error()})
			return
		}

		var getRequest voldriver.GetRequest
		if err = json.Unmarshal(body, &getRequest); err != nil {
			logger.Error("failed-unmarshalling-get-request-body", err)
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, voldriver.GetResponse{Err: err.Error()})
			return
		}

		getResponse := client.Get(ctx, getRequest)
		if getResponse.Err != "" {
			logger.Error("failed-getting-volume", err, lager.Data{"volume": getRequest.Name})
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, getResponse)
			return
		}

		cf_http_handlers.WriteJSONResponse(w, StatusOK, getResponse)
	}
}

func newListHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-list")
		logger.Info("start")
		defer logger.Info("end")

		ctx, cancel := withCancel(logger, req.Context(), w)
		defer cancel()

		listResponse := client.List(ctx)
		if listResponse.Err != "" {
			logger.Error("failed-listing-volumes", fmt.Errorf("%s", listResponse.Err))
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, listResponse)
			return
		}

		cf_http_handlers.WriteJSONResponse(w, StatusOK, listResponse)
	}
}

func newPathHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-path")
		logger.Info("start")
		defer logger.Info("end")

		ctx, cancel := withCancel(logger, req.Context(), w)
		defer cancel()

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logger.Error("failed-reading-path-request-body", err)
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, voldriver.MountResponse{Err: err.Error()})
			return
		}

		var pathRequest voldriver.PathRequest
		if err = json.Unmarshal(body, &pathRequest); err != nil {
			logger.Error("failed-unmarshalling-path-request-body", err)
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, voldriver.GetResponse{Err: err.Error()})
			return
		}

		pathResponse := client.Path(ctx, pathRequest)
		if pathResponse.Err != "" {
			logger.Error("failed-activating-driver", fmt.Errorf(pathResponse.Err))
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, pathResponse)
			return
		}

		cf_http_handlers.WriteJSONResponse(w, StatusOK, pathResponse)
	}
}

func newCapabilitiesHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-capabilities")
		logger.Info("start")
		defer logger.Info("end")

		capabilitiesResponse := client.Capabilities(logger)
		logger.Debug("capabilities-response", lager.Data{"capabilities": capabilitiesResponse})
		cf_http_handlers.WriteJSONResponse(w, StatusOK, capabilitiesResponse)
	}
}

func newCreateHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-create")
		logger.Info("start")
		defer logger.Info("end")

		ctx, cancel := withCancel(logger, req.Context(), w)
		defer cancel()

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logger.Error("failed-reading-create-request-body", err)
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, voldriver.ErrorResponse{Err: err.Error()})
			return
		}

		var createRequest voldriver.CreateRequest
		if err = json.Unmarshal(body, &createRequest); err != nil {
			logger.Error("failed-unmarshalling-create-request-body", err)
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, voldriver.ErrorResponse{Err: err.Error()})
			return
		}

		createResponse := client.Create(ctx, createRequest)
		if createResponse.Err != "" {
			logger.Error("failed-creating-volume", errors.New(createResponse.Err))
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, createResponse)
			return
		}

		cf_http_handlers.WriteJSONResponse(w, StatusOK, createResponse)
	}
}

func newMountHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-mount")
		logger.Info("start")
		defer logger.Info("end")

		ctx, cancel := withCancel(logger, req.Context(), w)
		defer cancel()

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logger.Error("failed-reading-mount-request-body", err)
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, voldriver.MountResponse{Err: err.Error()})
			return
		}

		var mountRequest voldriver.MountRequest
		if err = json.Unmarshal(body, &mountRequest); err != nil {
			logger.Error("failed-unmarshalling-mount-request-body", err)
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, voldriver.MountResponse{Err: err.Error()})
			return
		}

		mountResponse := client.Mount(ctx, mountRequest)
		if mountResponse.Err != "" {
			logger.Error("failed-mounting-volume", errors.New(mountResponse.Err), lager.Data{"volume": mountRequest.Name})
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, mountResponse)
			return
		}

		cf_http_handlers.WriteJSONResponse(w, StatusOK, mountResponse)
	}
}

func newUnmountHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-unmount")
		logger.Info("start")
		defer logger.Info("end")

		ctx, cancel := withCancel(logger, req.Context(), w)
		defer cancel()

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logger.Error("failed-reading-unmount-request-body", err)
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, voldriver.ErrorResponse{Err: err.Error()})
			return
		}

		var unmountRequest voldriver.UnmountRequest
		if err = json.Unmarshal(body, &unmountRequest); err != nil {
			logger.Error("failed-unmarshalling-unmount-request-body", err)
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, voldriver.ErrorResponse{Err: err.Error()})
			return
		}

		unmountResponse := client.Unmount(ctx, unmountRequest)
		if unmountResponse.Err != "" {
			logger.Error("failed-unmount-volume", errors.New(unmountResponse.Err), lager.Data{"volume": unmountRequest.Name})
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, unmountResponse)
			return
		}

		cf_http_handlers.WriteJSONResponse(w, StatusOK, unmountResponse)
	}
}

func newRemoveHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-remove")
		logger.Info("start")
		defer logger.Info("end")

		ctx, cancel := withCancel(logger, req.Context(), w)
		defer cancel()

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logger.Error("failed-reading-remove-request-body", err)
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, voldriver.ErrorResponse{Err: err.Error()})
			return
		}

		var removeRequest voldriver.RemoveRequest
		if err = json.Unmarshal(body, &removeRequest); err != nil {
			logger.Error("failed-unmarshalling-unmount-request-body", err)
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, voldriver.ErrorResponse{Err: err.Error()})
			return
		}

		removeResponse := client.Remove(ctx, removeRequest)
		if removeResponse.Err != "" {
			logger.Error("failed-remove-volume", errors.New(removeResponse.Err))
			cf_http_handlers.WriteJSONResponse(w, StatusInternalServerError, removeResponse)
			return
		}

		cf_http_handlers.WriteJSONResponse(w, StatusOK, removeResponse)
	}
}

func withCancel(logger lager.Logger, ctx context.Context, res http.ResponseWriter) (lager.Context, context.CancelFunc) {
	logger = logger.Session("with-cancel")
	logger.Info("start")
	defer logger.Info("end")

	newctx, cancel := lager.WithCancel(lager.NewContext(ctx, logger))

	if closer, ok := res.(http.CloseNotifier); ok {
		// Note: make calls in this thread to ensure reference on context
		doneOrTimeoutChannel := newctx.Done()
		cancelChannel := closer.CloseNotify()
		go func() {
			select {
			case <-doneOrTimeoutChannel:
			case <-cancelChannel:
				logger.Info("signalling cancel")
				cancel()
			}
		}()
	}

	return newctx, cancel
}
