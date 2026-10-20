package core

import (
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	minimaxFlashModel = "minimax/MiniMax-M3.1-Flash"
	minimaxM3Model    = "minimax/MiniMax-M3"
)

// ActiveMinimaxModels prefers the current text models returned by the provider.
// Legacy text IDs remain routable explicitly. Unrecognized and nontext models
// retain their metadata and relative order, and unavailable models are not added.
func ActiveMinimaxModels(models []types.Model) []types.Model {
	active := make([]types.Model, 0, len(models))
	for _, id := range []string{minimaxFlashModel, minimaxM3Model} {
		for _, model := range models {
			if model.ID == id {
				active = append(active, model)
			}
		}
	}
	for _, model := range models {
		switch model.ID {
		case minimaxFlashModel, minimaxM3Model,
			"minimax/MiniMax-M2", "minimax/MiniMax-M2.1", "minimax/MiniMax-M2.1-highspeed",
			"minimax/MiniMax-M2.5", "minimax/MiniMax-M2.5-highspeed",
			"minimax/MiniMax-M2.7", "minimax/MiniMax-M2.7-highspeed":
			continue
		default:
			active = append(active, model)
		}
	}
	return active
}
