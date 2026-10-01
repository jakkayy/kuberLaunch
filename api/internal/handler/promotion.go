package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jakkayy/kuberlauncher/api/internal/auth"
	"github.com/jakkayy/kuberlauncher/api/internal/model"
	"github.com/jakkayy/kuberlauncher/api/internal/service"
)

type PromotionHandler struct {
	svc *service.PromotionService
}

func NewPromotionHandler(svc *service.PromotionService) *PromotionHandler {
	return &PromotionHandler{svc: svc}
}

// Promote promotes the previous environment's image into :env.
// POST /api/v1/projects/:id/environments/:env/promote
func (h *PromotionHandler) Promote(c *gin.Context) {
	pr, err := h.svc.Promote(c.Request.Context(), auth.UserFrom(c), c.Param("id"), model.EnvName(c.Param("env")))
	if err != nil {
		respondError(c, err)
		return
	}
	status := http.StatusCreated
	if pr.Status == model.PromotionPending {
		status = http.StatusAccepted
	}
	c.JSON(status, pr)
}

// GET /api/v1/projects/:id/promotions
func (h *PromotionHandler) List(c *gin.Context) {
	list, err := h.svc.List(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	if list == nil {
		list = []model.Promotion{}
	}
	c.JSON(http.StatusOK, gin.H{"promotions": list})
}

// POST /api/v1/projects/:id/promotions/:promotion_id/approve
func (h *PromotionHandler) Approve(c *gin.Context) {
	pr, err := h.svc.Approve(c.Request.Context(), auth.UserFrom(c), c.Param("id"), c.Param("promotion_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, pr)
}

// POST /api/v1/projects/:id/promotions/:promotion_id/reject
func (h *PromotionHandler) Reject(c *gin.Context) {
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	pr, err := h.svc.Reject(c.Request.Context(), auth.UserFrom(c), c.Param("id"), c.Param("promotion_id"), req.Reason)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, pr)
}
