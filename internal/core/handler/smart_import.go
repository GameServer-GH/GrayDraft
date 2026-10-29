package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/observability"
	"github.com/graydraft/core/service/kbscope"
	"github.com/graydraft/core/service/smartimport"
)

// SmartImportHandler handles agent-assisted bulk import.
type SmartImportHandler struct {
	svc      *smartimport.Service
	resolver *kbscope.Resolver
}

// NewSmartImportHandler creates the handler.
func NewSmartImportHandler(svc *smartimport.Service, resolver *kbscope.Resolver) *SmartImportHandler {
	return &SmartImportHandler{svc: svc, resolver: resolver}
}

// Import handles POST /api/v1/ugc/smart-import
func (h *SmartImportHandler) Import(c *gin.Context) {
	var req model.SmartImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":                   err.Error(),
			"confidence":              0,
			"min_confidence_required": 0,
			"import_allowed":          false,
		})
		return
	}
	kbType, err := ResolveKBType(c, h.resolver, req.TenantID, req.KBID, req.HintKBType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":                   err.Error(),
			"confidence":              0,
			"min_confidence_required": 0,
			"import_allowed":          false,
		})
		return
	}
	if smartimport.IsWorkspacePartition(req.HintKBType) {
		// GrayDraft workspace partition always wins over PaaS-resolved legacy types.
	} else if req.HintKBType == "" {
		req.HintKBType = kbType
	}

	resp, err := h.svc.Run(c.Request.Context(), req)
	channel := ""
	if resp != nil && len(resp.Tasks) > 0 {
		channel = resp.Tasks[0].Channel
	} else if resp != nil && resp.Task != nil {
		channel = resp.Task.Channel
	}
	observability.RecordUGCNotify(channel, err)

	var confErr *smartimport.ErrBelowMinConfidence
	if errors.As(err, &confErr) && confErr.Response != nil {
		c.JSON(http.StatusUnprocessableEntity, confErr.Response)
		return
	}
	if err != nil {
		body := gin.H{
			"error":          err.Error(),
			"confidence":     0,
			"import_allowed": false,
		}
		if resp != nil {
			body = gin.H{
				"error":                   err.Error(),
				"confidence":              resp.Confidence,
				"min_confidence_required": resp.MinConfidenceRequired,
				"import_allowed":          resp.ImportAllowed,
				"split":                   resp.Split,
				"preview":                 resp.Preview,
				"items":                   resp.Items,
				"message":                 resp.Message,
			}
		}
		c.JSON(http.StatusBadRequest, body)
		return
	}

	if req.Submit && len(resp.Tasks) > 0 {
		c.JSON(http.StatusAccepted, resp)
		return
	}
	if req.Submit && resp.Task != nil {
		c.JSON(http.StatusAccepted, resp)
		return
	}
	c.JSON(http.StatusOK, resp)
}
