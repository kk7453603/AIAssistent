package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type runtimeModelsRequest struct {
	GenModel       *string `json:"gen_model"`
	PlannerModel   *string `json:"planner_model"`
	EmbeddingModel *string `json:"embedding_model"`
}

type runtimeModelsResponse struct {
	GenModel              string `json:"gen_model"`
	PlannerModel          string `json:"planner_model"`
	EmbeddingModel        string `json:"embedding_model"`
	Provider              string `json:"provider"`
	RuntimeApplySupported bool   `json:"runtime_apply_supported"`
}

func (rt *Router) handleGetRuntimeModels(w http.ResponseWriter, _ *http.Request) {
	if rt.runtimeModelConfig == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("runtime model apply is not configured"))
		return
	}

	writeJSON(w, http.StatusOK, toRuntimeModelsResponse(rt.runtimeModelConfig.GetRuntimeModelConfig()))
}

func (rt *Router) handlePutRuntimeModels(w http.ResponseWriter, r *http.Request) {
	if rt.runtimeModelConfig == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("runtime model apply is not configured"))
		return
	}

	var req runtimeModelsRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	next := rt.runtimeModelConfig.GetRuntimeModelConfig()
	if req.GenModel != nil {
		next.GenerationModel = strings.TrimSpace(*req.GenModel)
	}
	if req.PlannerModel != nil {
		next.PlannerModel = strings.TrimSpace(*req.PlannerModel)
	}
	if req.EmbeddingModel != nil {
		next.EmbeddingModel = strings.TrimSpace(*req.EmbeddingModel)
	}

	if err := rt.runtimeModelConfig.SetRuntimeModelConfig(next); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, toRuntimeModelsResponse(rt.runtimeModelConfig.GetRuntimeModelConfig()))
}

func toRuntimeModelsResponse(config ports.RuntimeModelConfig) runtimeModelsResponse {
	return runtimeModelsResponse{
		GenModel:              config.GenerationModel,
		PlannerModel:          config.PlannerModel,
		EmbeddingModel:        config.EmbeddingModel,
		Provider:              "ollama",
		RuntimeApplySupported: true,
	}
}
