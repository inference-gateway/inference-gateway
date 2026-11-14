package api

import (
	"net/http"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
)

type ResponseJSON struct {
	Message string `json:"message"`
}

func (router *RouterImpl) HealthcheckHandler(w http.ResponseWriter, r *http.Request) {
	router.logger.Debug("healthcheck")
	middlewares.WriteJSON(w, http.StatusOK, ResponseJSON{Message: "OK"})
}
