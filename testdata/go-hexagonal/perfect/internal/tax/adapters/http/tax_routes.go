package http

import "net/http"

func RegisterTaxRoutes(mux *http.ServeMux, handler *TaxHandler) {
	mux.HandleFunc("GET /tax-rules", handler.ListRules)
}
