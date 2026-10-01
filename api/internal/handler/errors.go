package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

// respondError writes err as JSON with a status code derived from service sentinel errors.
func respondError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrNotFound), err.Error() == "project not found":
		status = http.StatusNotFound
	case errors.Is(err, service.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, service.ErrForbidden):
		status = http.StatusForbidden
	}
	c.JSON(status, gin.H{"error": err.Error()})
}
