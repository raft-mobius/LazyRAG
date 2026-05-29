package chat

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gorm.io/gorm"

	"lazymind/core/modelconfig"
	"lazymind/core/modelprovider"
)

type selectedRuntimeModel = modelconfig.SelectedRuntimeModel

func loadLLMConfig(ctx context.Context, db *gorm.DB, userID string) (map[string]any, error) {
	if strings.TrimSpace(os.Getenv("LAZYMIND_MODE")) == "desktop" {
		config := modelprovider.BuildLLMConfigFromFile()
		fmt.Printf("[Core] [LLM_CONFIG_LOADED] [mode=desktop] [user_id=%s] [%s]\n", strings.TrimSpace(userID), modelconfig.SummarizeLLMConfigForLog(config))
		return config, nil
	}

	return modelconfig.LoadLLMConfig(ctx, db, userID)
}

func buildLLMConfig(rows []selectedRuntimeModel) map[string]any {
	return modelconfig.BuildLLMConfig(rows)
}
