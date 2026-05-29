package modelprovider

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"lazymind/core/common"
	"lazymind/core/log"
)

// --- List providers ---

func listUserProvidersFromFile(w http.ResponseWriter, r *http.Request) {
	cfg := getFileConfig()
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))

	out := make([]listItem, 0, len(cfg.Providers))
	for _, p := range cfg.Providers {
		if keyword != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(keyword)) {
			continue
		}
		out = append(out, listItem{
			ID:                     p.ID,
			DefaultModelProviderID: p.ID,
			Name:                   p.Name,
			Description:            p.Description,
			BaseURL:                p.BaseURL,
		})
	}
	common.ReplyOK(w, listResponse{Providers: out})
}

func listUserProvidersWithGroupsFromFile(w http.ResponseWriter, r *http.Request) {
	cfg := getFileConfig()

	out := make([]listItem, 0)
	for _, p := range cfg.Providers {
		if len(p.Groups) > 0 {
			out = append(out, listItem{
				ID:                     p.ID,
				DefaultModelProviderID: p.ID,
				Name:                   p.Name,
				Description:            p.Description,
				BaseURL:                p.BaseURL,
			})
		}
	}
	common.ReplyOK(w, listResponse{Providers: out})
}

// --- Groups ---

func listGroupsFromFile(w http.ResponseWriter, r *http.Request) {
	parentID := strings.TrimSpace(mux.Vars(r)["model_provider_id"])
	if parentID == "" {
		common.ReplyErr(w, "missing model_provider_id", http.StatusBadRequest)
		return
	}

	cfg := getFileConfig()
	provider := findProvider(cfg, parentID)
	if provider == nil {
		common.ReplyErr(w, "model provider not found", http.StatusNotFound)
		return
	}

	out := make([]groupListItem, 0, len(provider.Groups))
	for _, g := range provider.Groups {
		out = append(out, groupListItem{
			ID:                  g.ID,
			UserModelProviderID: provider.ID,
			Name:                g.Name,
			BaseURL:             g.BaseURL,
			APIKey:              g.APIKey,
			IsVerified:          g.IsVerified,
		})
	}
	common.ReplyOK(w, groupListResponse{Groups: out})
}

func createGroupFromFile(w http.ResponseWriter, r *http.Request) {
	parentID := strings.TrimSpace(mux.Vars(r)["model_provider_id"])
	if parentID == "" {
		common.ReplyErr(w, "missing model_provider_id", http.StatusBadRequest)
		return
	}

	var req createGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.ReplyErr(w, "invalid body", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(req.Name)
	baseURL := strings.TrimSpace(req.BaseURL)
	apiKey := strings.TrimSpace(req.APIKey)
	if name == "" || baseURL == "" {
		common.ReplyErr(w, "name and base_url are required", http.StatusBadRequest)
		return
	}

	newGroup := FileGroup{
		ID:         common.GenerateID(),
		Name:       name,
		BaseURL:    baseURL,
		APIKey:     apiKey,
		IsVerified: false,
		Models:     []FileModel{},
	}

	var providerName string
	if err := updateFileConfig(func(cfg *FileModelConfig) {
		for i := range cfg.Providers {
			if cfg.Providers[i].ID == parentID {
				cfg.Providers[i].Groups = append(cfg.Providers[i].Groups, newGroup)
				providerName = cfg.Providers[i].Name
				return
			}
		}
	}); err != nil {
		common.ReplyErr(w, "save config failed", http.StatusInternalServerError)
		return
	}
	if providerName == "" {
		common.ReplyErr(w, "model provider not found", http.StatusNotFound)
		return
	}

	common.ReplyOK(w, createGroupResponse{
		ID:                  newGroup.ID,
		UserModelProviderID: parentID,
		Name:                newGroup.Name,
		BaseURL:             newGroup.BaseURL,
	})
}

func updateGroupFromFile(w http.ResponseWriter, r *http.Request) {
	parentID := strings.TrimSpace(mux.Vars(r)["model_provider_id"])
	groupID := strings.TrimSpace(mux.Vars(r)["group_id"])
	if parentID == "" || groupID == "" {
		common.ReplyErr(w, "missing model_provider_id or group_id", http.StatusBadRequest)
		return
	}

	var req updateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.ReplyErr(w, "invalid body", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(req.Name)
	baseURL := strings.TrimSpace(req.BaseURL)
	apiKey := strings.TrimSpace(req.APIKey)
	if name == "" || baseURL == "" {
		common.ReplyErr(w, "name and base_url are required", http.StatusBadRequest)
		return
	}

	found := false
	if err := updateFileConfig(func(cfg *FileModelConfig) {
		p := findProviderMut(cfg, parentID)
		if p == nil {
			return
		}
		for i := range p.Groups {
			if p.Groups[i].ID == groupID {
				if baseURL != p.Groups[i].BaseURL {
					p.Groups[i].IsVerified = false
				}
				p.Groups[i].Name = name
				p.Groups[i].BaseURL = baseURL
				if apiKey != "" {
					if apiKey != p.Groups[i].APIKey {
						p.Groups[i].IsVerified = false
					}
					p.Groups[i].APIKey = apiKey
				}
				found = true
				return
			}
		}
	}); err != nil {
		common.ReplyErr(w, "save config failed", http.StatusInternalServerError)
		return
	}
	if !found {
		common.ReplyErr(w, "group not found", http.StatusNotFound)
		return
	}

	common.ReplyOK(w, createGroupResponse{
		ID:                  groupID,
		UserModelProviderID: parentID,
		Name:                name,
		BaseURL:             baseURL,
	})
}

func deleteGroupFromFile(w http.ResponseWriter, r *http.Request) {
	parentID := strings.TrimSpace(mux.Vars(r)["model_provider_id"])
	groupID := strings.TrimSpace(mux.Vars(r)["group_id"])
	if parentID == "" || groupID == "" {
		common.ReplyErr(w, "missing model_provider_id or group_id", http.StatusBadRequest)
		return
	}

	found := false
	if err := updateFileConfig(func(cfg *FileModelConfig) {
		p := findProviderMut(cfg, parentID)
		if p == nil {
			return
		}
		for i := range p.Groups {
			if p.Groups[i].ID == groupID {
				p.Groups = append(p.Groups[:i], p.Groups[i+1:]...)
				found = true
				return
			}
		}
	}); err != nil {
		common.ReplyErr(w, "save config failed", http.StatusInternalServerError)
		return
	}
	if !found {
		common.ReplyErr(w, "group not found", http.StatusNotFound)
		return
	}

	common.ReplyOK(w, deleteGroupResponse{ID: groupID})
}

// --- Group models ---

func listGroupModelsFromFile(w http.ResponseWriter, r *http.Request) {
	parentID := strings.TrimSpace(mux.Vars(r)["model_provider_id"])
	groupID := strings.TrimSpace(mux.Vars(r)["group_id"])
	if parentID == "" || groupID == "" {
		common.ReplyErr(w, "missing model_provider_id or group_id", http.StatusBadRequest)
		return
	}

	cfg := getFileConfig()
	provider := findProvider(cfg, parentID)
	if provider == nil {
		common.ReplyErr(w, "model provider not found", http.StatusNotFound)
		return
	}
	group := findGroup(provider, groupID)
	if group == nil {
		common.ReplyErr(w, "group not found", http.StatusNotFound)
		return
	}

	out := make([]groupModelListItem, 0, len(group.Models))
	for _, m := range group.Models {
		out = append(out, groupModelListItem{
			ID:                       m.ID,
			UserModelProviderID:      provider.ID,
			UserModelProviderGroupID: group.ID,
			Name:                     m.Name,
			ModelType:                m.ModelType,
			ProviderName:             provider.Name,
			GroupName:                group.Name,
			BaseURL:                  effectiveBaseURL(m.BaseURL, group.BaseURL),
			IsDefault:                m.IsDefault,
		})
	}
	common.ReplyOK(w, groupModelListResponse{Models: out})
}

func addGroupModelFromFile(w http.ResponseWriter, r *http.Request) {
	parentID := strings.TrimSpace(mux.Vars(r)["model_provider_id"])
	groupID := strings.TrimSpace(mux.Vars(r)["group_id"])
	if parentID == "" || groupID == "" {
		common.ReplyErr(w, "missing model_provider_id or group_id", http.StatusBadRequest)
		return
	}

	var req addGroupModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.ReplyErr(w, "invalid body", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(req.Name)
	modelType := strings.TrimSpace(req.ModelType)
	if name == "" || modelType == "" {
		common.ReplyErr(w, "name and model_type are required", http.StatusBadRequest)
		return
	}

	newModel := FileModel{
		ID:        common.GenerateID(),
		Name:      name,
		ModelType: modelType,
		BaseURL:   "",
		IsDefault: false,
	}

	var resp *addGroupModelResponse
	if err := updateFileConfig(func(cfg *FileModelConfig) {
		p := findProviderMut(cfg, parentID)
		if p == nil {
			return
		}
		g := findGroupMut(p, groupID)
		if g == nil {
			return
		}
		for _, m := range g.Models {
			if m.Name == name {
				return
			}
		}
		g.Models = append(g.Models, newModel)
		resp = &addGroupModelResponse{
			ID:                       newModel.ID,
			UserModelProviderID:      p.ID,
			UserModelProviderGroupID: g.ID,
			Name:                     newModel.Name,
			ModelType:                newModel.ModelType,
			ProviderName:             p.Name,
			GroupName:                g.Name,
			BaseURL:                  effectiveBaseURL(newModel.BaseURL, g.BaseURL),
			IsDefault:                newModel.IsDefault,
		}
	}); err != nil {
		common.ReplyErr(w, "save config failed", http.StatusInternalServerError)
		return
	}
	if resp == nil {
		common.ReplyErr(w, "provider, group, or duplicate model name", http.StatusBadRequest)
		return
	}
	common.ReplyOK(w, resp)
}

func deleteGroupModelFromFile(w http.ResponseWriter, r *http.Request) {
	parentID := strings.TrimSpace(mux.Vars(r)["model_provider_id"])
	groupID := strings.TrimSpace(mux.Vars(r)["group_id"])
	modelID := strings.TrimSpace(mux.Vars(r)["model_id"])
	if parentID == "" || groupID == "" || modelID == "" {
		common.ReplyErr(w, "missing model_provider_id, group_id, or model_id", http.StatusBadRequest)
		return
	}

	found := false
	if err := updateFileConfig(func(cfg *FileModelConfig) {
		p := findProviderMut(cfg, parentID)
		if p == nil {
			return
		}
		g := findGroupMut(p, groupID)
		if g == nil {
			return
		}
		for i := range g.Models {
			if g.Models[i].ID == modelID {
				g.Models = append(g.Models[:i], g.Models[i+1:]...)
				found = true
				return
			}
		}
	}); err != nil {
		common.ReplyErr(w, "save config failed", http.StatusInternalServerError)
		return
	}
	if !found {
		common.ReplyErr(w, "model not found", http.StatusNotFound)
		return
	}
	common.ReplyOK(w, deleteGroupModelResponse{ID: modelID})
}

func listUserModelsByModelTypeFromFile(w http.ResponseWriter, r *http.Request) {
	modelType := strings.TrimSpace(r.URL.Query().Get("model_type"))
	if modelType == "" {
		common.ReplyErr(w, "model_type is required", http.StatusBadRequest)
		return
	}

	cfg := getFileConfig()
	out := make([]groupModelListItem, 0)
	for _, p := range cfg.Providers {
		for _, g := range p.Groups {
			for _, m := range g.Models {
				if m.ModelType == modelType {
					out = append(out, groupModelListItem{
						ID:                       m.ID,
						UserModelProviderID:      p.ID,
						UserModelProviderGroupID: g.ID,
						Name:                     m.Name,
						ModelType:                m.ModelType,
						ProviderName:             p.Name,
						GroupName:                g.Name,
						BaseURL:                  effectiveBaseURL(m.BaseURL, g.BaseURL),
						IsDefault:                m.IsDefault,
					})
				}
			}
		}
	}
	common.ReplyOK(w, groupModelListResponse{Models: out})
}

// --- Selection ---

func getSelectedModelsFromFile(w http.ResponseWriter, r *http.Request) {
	cfg := getFileConfig()

	type modelInfo struct {
		providerID string
		groupID    string
		name       string
		provider   string
		groupName  string
		baseURL    string
	}
	index := make(map[string]modelInfo)
	for _, p := range cfg.Providers {
		for _, g := range p.Groups {
			for _, m := range g.Models {
				index[m.ID] = modelInfo{
					providerID: p.ID,
					groupID:    g.ID,
					name:       m.Name,
					provider:   p.Name,
					groupName:  g.Name,
					baseURL:    effectiveBaseURL(m.BaseURL, g.BaseURL),
				}
			}
		}
	}

	out := make([]selectedModelItem, 0, len(cfg.SelectedModels))
	for _, sel := range cfg.SelectedModels {
		info, ok := index[sel.ModelID]
		if !ok {
			continue
		}
		out = append(out, selectedModelItem{
			ModelType:                sel.ModelType,
			ModelID:                  sel.ModelID,
			UserModelProviderID:      info.providerID,
			UserModelProviderGroupID: info.groupID,
			Name:                     info.name,
			ProviderName:             info.provider,
			GroupName:                info.groupName,
			BaseURL:                  info.baseURL,
		})
	}
	common.ReplyOK(w, selectedModelsResponse{Selections: out})
}

func setSelectedModelsFromFile(w http.ResponseWriter, r *http.Request) {
	var req setSelectedModelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.ReplyErr(w, "invalid body", http.StatusBadRequest)
		return
	}
	if len(req.Selections) == 0 {
		common.ReplyErr(w, "selections required", http.StatusBadRequest)
		return
	}

	for _, item := range req.Selections {
		modelType := strings.TrimSpace(item.ModelType)
		if modelType == "" {
			common.ReplyErr(w, "model_type is required", http.StatusBadRequest)
			return
		}
		if _, ok := allowedSelectionModelTypes[modelType]; !ok {
			common.ReplyErr(w, "invalid model_type", http.StatusBadRequest)
			return
		}
	}

	if err := updateFileConfig(func(cfg *FileModelConfig) {
		for _, item := range req.Selections {
			modelType := strings.TrimSpace(item.ModelType)
			modelID := strings.TrimSpace(item.ModelID)

			found := false
			for i := range cfg.SelectedModels {
				if cfg.SelectedModels[i].ModelType == modelType {
					if modelID == "" {
						cfg.SelectedModels = append(cfg.SelectedModels[:i], cfg.SelectedModels[i+1:]...)
					} else {
						cfg.SelectedModels[i].ModelID = modelID
					}
					found = true
					break
				}
			}
			if !found && modelID != "" {
				cfg.SelectedModels = append(cfg.SelectedModels, FileSelectedModel{
					ModelType: modelType,
					ModelID:   modelID,
				})
			}
		}
	}); err != nil {
		common.ReplyErr(w, "save config failed", http.StatusInternalServerError)
		return
	}

	getSelectedModelsFromFile(w, r)
}

// --- Check group ---

func checkGroupFromFile(w http.ResponseWriter, r *http.Request) {
	var req checkModelProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.ReplyErr(w, "invalid body", http.StatusBadRequest)
		return
	}
	source := strings.TrimSpace(req.ProviderName)
	urlStr := strings.TrimSpace(req.BaseURL)
	apiKey := strings.TrimSpace(req.APIKey)
	if source == "" || urlStr == "" || apiKey == "" {
		common.ReplyErr(w, "provider_name, base_url, and api_key are required", http.StatusBadRequest)
		return
	}

	parentID := strings.TrimSpace(mux.Vars(r)["model_provider_id"])
	groupID := strings.TrimSpace(mux.Vars(r)["group_id"])
	if parentID == "" || groupID == "" {
		common.ReplyErr(w, "missing model_provider_id or group_id", http.StatusBadRequest)
		return
	}

	upstream := common.JoinURL(common.ChatServiceEndpoint(), "/api/model/check")
	body := algoModelCheckBody{
		Source: source,
		URL:    urlStr,
		APIKey: apiKey,
	}
	checkStart := time.Now()

	var algo modelCheckResponse
	if err := common.ApiPost(r.Context(), upstream, body, nil, &algo, modelProviderCheckTimeout); err != nil {
		log.Logger.Error().
			Err(err).
			Str("upstream", upstream).
			Str("provider_name", source).
			Str("base_url", urlStr).
			Dur("elapsed", time.Since(checkStart)).
			Msg("model provider check failed (desktop file mode)")
		common.ReplyErrWithData(w, err.Error(), algo, http.StatusBadGateway)
		return
	}

	if algo.Success {
		_ = updateFileConfig(func(cfg *FileModelConfig) {
			p := findProviderMut(cfg, parentID)
			if p == nil {
				return
			}
			for i := range p.Groups {
				if p.Groups[i].ID == groupID {
					p.Groups[i].IsVerified = true
					return
				}
			}
		})
	}
	common.ReplyOK(w, CheckModelProviderData{Success: algo.Success, Message: algo.Message})
}

// --- Helpers ---

func findProvider(cfg *FileModelConfig, id string) *FileProvider {
	for i := range cfg.Providers {
		if cfg.Providers[i].ID == id {
			return &cfg.Providers[i]
		}
	}
	return nil
}

func findProviderMut(cfg *FileModelConfig, id string) *FileProvider {
	return findProvider(cfg, id)
}

func findGroup(p *FileProvider, id string) *FileGroup {
	for i := range p.Groups {
		if p.Groups[i].ID == id {
			return &p.Groups[i]
		}
	}
	return nil
}

func findGroupMut(p *FileProvider, id string) *FileGroup {
	return findGroup(p, id)
}

func effectiveBaseURL(modelURL, groupURL string) string {
	if modelURL != "" {
		return modelURL
	}
	return groupURL
}
