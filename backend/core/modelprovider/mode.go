package modelprovider

import (
	"os"
	"strings"
)

func isDesktopMode() bool {
	return strings.TrimSpace(os.Getenv("LAZYMIND_MODE")) == "desktop"
}

func modelConfigFilePath() string {
	return strings.TrimSpace(os.Getenv("LAZYMIND_MODEL_CONFIG_FILE"))
}
