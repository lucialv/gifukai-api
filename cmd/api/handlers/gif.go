package handlers

import (
	"fmt"
	"net/http"

	"github.com/lucialv/gifukai-api/pkg/store"
	u "github.com/lucialv/gifukai-api/pkg/utils"

	"github.com/go-chi/chi/v5"
)

type actionCoverageItem struct {
	Action       string               `json:"action"`
	Total        int                  `json:"total"`
	ByPairing    []store.PairingCount `json:"by_pairing"`
	PairingCount *int                 `json:"pairing_count,omitempty"`
}

type actionItem struct {
	Action   string `json:"action"`
	HasTypes bool   `json:"has_types"`
}

func (h *Handler) ListActionsHandler(w http.ResponseWriter, r *http.Request) error {
	actions, err := h.Store.GetAllActions()
	if err != nil {
		return err
	}
	actions = u.OrEmpty(actions)

	items := make([]actionItem, 0, len(actions))
	for _, action := range actions {
		hasTypes, err := h.Store.ActionHasTypes(action)
		if err != nil {
			return err
		}
		items = append(items, actionItem{Action: action, HasTypes: hasTypes})
	}

	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"actions":            actions,
		"actions_with_types": items,
	})
}

func (h *Handler) ActionTypesHandler(w http.ResponseWriter, r *http.Request) error {
	action := NormalizeAction(chi.URLParam(r, "action"))
	if action == "" {
		u.WriteError(w, http.StatusBadRequest, "action is required")
		return nil
	}

	types, hasTypes, err := h.Store.GetActionTypes(action)
	if err != nil {
		return err
	}

	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"action":    action,
		"has_types": hasTypes,
		"types":     u.OrEmpty(types),
	})
}

func (h *Handler) ActionVariantsAdminHandler(w http.ResponseWriter, r *http.Request) error {
	action := NormalizeAction(chi.URLParam(r, "action"))
	if action == "" {
		u.WriteError(w, http.StatusBadRequest, "action is required")
		return nil
	}

	variants, err := h.Store.ListActionVariants(action)
	if err != nil {
		return err
	}
	hasTypes, err := h.Store.ActionHasTypes(action)
	if err != nil {
		return err
	}

	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"action":    action,
		"has_types": hasTypes,
		"types":     u.OrEmpty(variants),
	})
}

func (h *Handler) ActionCoverageHandler(w http.ResponseWriter, r *http.Request) error {
	pairing := NormalizePairing(r.URL.Query().Get("pairing"))
	if pairing != "" && !ValidPairings[pairing] {
		u.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid pairing: %s (valid: f, m, ff, mm, fm, mf)", pairing))
		return nil
	}

	actions, err := h.Store.GetAllActions()
	if err != nil {
		return err
	}
	counts, err := h.Store.GetActionPairingCounts()
	if err != nil {
		return err
	}

	totalsByAction := make(map[string]int, len(actions))
	countsByAction := make(map[string]map[string]int, len(actions))
	for _, row := range counts {
		action := row.Action
		totalsByAction[action] += row.Count
		if _, ok := countsByAction[action]; !ok {
			countsByAction[action] = make(map[string]int)
		}
		countsByAction[action][row.Pairing] = row.Count
	}

	items := make([]actionCoverageItem, 0, len(actions))
	for _, action := range actions {
		byPairing := make([]store.PairingCount, 0, len(ValidPairings))
		for _, p := range PairingOrder {
			pairingCount := 0
			if m, ok := countsByAction[action]; ok {
				pairingCount = m[p]
			}

			byPairing = append(byPairing, store.PairingCount{Pairing: p, Count: pairingCount})
		}

		item := actionCoverageItem{
			Action:    action,
			Total:     totalsByAction[action],
			ByPairing: byPairing,
		}
		if pairing != "" {
			v := 0
			if m, ok := countsByAction[action]; ok {
				v = m[pairing]
			}
			item.PairingCount = &v
		}

		items = append(items, item)
	}

	return u.WriteJSON(w, http.StatusOK, map[string]any{
		"pairing": pairing,
		"actions": items,
	})
}

func (h *Handler) CountGifsHandler(w http.ResponseWriter, r *http.Request) error {
	policy, badRequest, err := ResolveActionTypePolicy(h.Store, chi.URLParam(r, "action"), r.URL.Query().Get("type"), false)
	if err != nil {
		return err
	}
	if badRequest != "" {
		u.WriteError(w, http.StatusBadRequest, badRequest)
		return nil
	}
	if policy.Action == "" {
		u.WriteError(w, http.StatusBadRequest, "action is required")
		return nil
	}

	count, err := h.Store.CountGifs(policy.Action, "", policy.Type)
	if err != nil {
		return err
	}

	byPairing, err := h.Store.CountGifsByPairing(policy.Action, policy.Type)
	if err != nil {
		return err
	}
	byPairing = u.OrEmpty(byPairing)

	response := map[string]any{
		"action":     policy.Action,
		"count":      count,
		"by_pairing": byPairing,
		"has_types":  policy.HasTypes,
	}
	if policy.Type != "" {
		response["type"] = policy.Type
	}

	return u.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) GetRandomGifHandler(w http.ResponseWriter, r *http.Request) error {
	policy, badRequest, err := ResolveActionTypePolicy(h.Store, chi.URLParam(r, "action"), r.URL.Query().Get("type"), false)
	if err != nil {
		return err
	}
	if badRequest != "" {
		u.WriteError(w, http.StatusBadRequest, badRequest)
		return nil
	}
	if policy.Action == "" {
		u.WriteError(w, http.StatusBadRequest, "action is required")
		return nil
	}

	pairing := NormalizePairing(r.URL.Query().Get("pairing"))
	if pairing != "" && !ValidPairings[pairing] {
		u.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid pairing: %s (valid: f, m, ff, mm, fm, mf)", pairing))
		return nil
	}

	var nsfwFilter *bool
	if nsfwParam := r.URL.Query().Get("nsfw"); nsfwParam != "" {
		val := nsfwParam == "true" || nsfwParam == "1"
		nsfwFilter = &val
	} else {
		val := false
		nsfwFilter = &val
	}

	var (
		result *store.Gif
	)
	if pairing == "" {
		result, err = h.Store.GetRandomGifAnyPairing(policy.Action, policy.Type, nsfwFilter)
	} else {
		result, err = h.Store.GetRandomGif(policy.Action, pairing, policy.Type, nsfwFilter)
	}
	if err != nil {
		return err
	}
	if result == nil {
		msg := fmt.Sprintf("no GIFs found for action: %s", policy.Action)
		if pairing != "" {
			msg = fmt.Sprintf("no GIFs found for action: %s, pairing: %s", policy.Action, pairing)
		}
		if policy.Type != "" {
			msg = fmt.Sprintf("%s, type: %s", msg, policy.Type)
		}
		u.WriteError(w, http.StatusNotFound, msg)
		return nil
	}

	resp := h.buildGifResponse(result)
	HideVariantIfUntyped(&resp, policy.HasTypes)
	return u.WriteJSON(w, http.StatusOK, resp)
}
