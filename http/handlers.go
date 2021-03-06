// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package http

import (
	"fmt"
	"net/http"

	auth "github.com/abbot/go-http-auth"
	"github.com/golang/glog"
	"github.com/Clever/cadvisor/api"
	"github.com/Clever/cadvisor/healthz"
	httpMux "github.com/Clever/cadvisor/http/mux"
	"github.com/Clever/cadvisor/manager"
	"github.com/Clever/cadvisor/metrics"
	"github.com/Clever/cadvisor/pages"
	"github.com/Clever/cadvisor/pages/static"
	"github.com/Clever/cadvisor/validate"
	"github.com/prometheus/client_golang/prometheus"
)

func RegisterHandlers(mux httpMux.Mux, containerManager manager.Manager, httpAuthFile, httpAuthRealm, httpDigestFile, httpDigestRealm, prometheusEndpoint string) error {
	// Basic health handler.
	if err := healthz.RegisterHandler(mux); err != nil {
		return fmt.Errorf("failed to register healthz handler: %s", err)
	}

	// Validation/Debug handler.
	mux.HandleFunc(validate.ValidatePage, func(w http.ResponseWriter, r *http.Request) {
		err := validate.HandleRequest(w, containerManager)
		if err != nil {
			fmt.Fprintf(w, "%s", err)
		}
	})

	// Register API handler.
	if err := api.RegisterHandlers(mux, containerManager); err != nil {
		return fmt.Errorf("failed to register API handlers: %s", err)
	}

	// Redirect / to containers page.
	mux.Handle("/", http.RedirectHandler(pages.ContainersPage, http.StatusTemporaryRedirect))

	var authenticated bool = false

	// Setup the authenticator object
	if httpAuthFile != "" {
		glog.Infof("Using auth file %s", httpAuthFile)
		secrets := auth.HtpasswdFileProvider(httpAuthFile)
		authenticator := auth.NewBasicAuthenticator(httpAuthRealm, secrets)
		mux.HandleFunc(static.StaticResource, authenticator.Wrap(staticHandler))
		if err := pages.RegisterHandlersBasic(mux, containerManager, authenticator); err != nil {
			return fmt.Errorf("failed to register pages auth handlers: %s", err)
		}
		authenticated = true
	}
	if httpAuthFile == "" && httpDigestFile != "" {
		glog.Infof("Using digest file %s", httpDigestFile)
		secrets := auth.HtdigestFileProvider(httpDigestFile)
		authenticator := auth.NewDigestAuthenticator(httpDigestRealm, secrets)
		mux.HandleFunc(static.StaticResource, authenticator.Wrap(staticHandler))
		if err := pages.RegisterHandlersDigest(mux, containerManager, authenticator); err != nil {
			return fmt.Errorf("failed to register pages digest handlers: %s", err)
		}
		authenticated = true
	}

	// Change handler based on authenticator initalization
	if !authenticated {
		mux.HandleFunc(static.StaticResource, staticHandlerNoAuth)
		if err := pages.RegisterHandlersBasic(mux, containerManager, nil); err != nil {
			return fmt.Errorf("failed to register pages handlers: %s", err)
		}
	}

	collector := metrics.NewPrometheusCollector(containerManager)
	prometheus.MustRegister(collector)
	http.Handle(prometheusEndpoint, prometheus.Handler())

	return nil
}

func staticHandlerNoAuth(w http.ResponseWriter, r *http.Request) {
	err := static.HandleRequest(w, r.URL)
	if err != nil {
		fmt.Fprintf(w, "%s", err)
	}
}

func staticHandler(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	err := static.HandleRequest(w, r.URL)
	if err != nil {
		fmt.Fprintf(w, "%s", err)
	}
}
