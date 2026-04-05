package env

import (
	"os"
	"strings"
)

func Load() {
	envFileName := ".env"
	envF, err := os.ReadFile(envFileName)
	if err != nil {
		return
	}
	envArr := strings.SplitSeq(string(envF), "\n")
	for envP := range envArr {
		envP = strings.TrimSpace(envP)
		if len(envP) == 0 || strings.HasPrefix(envP, "#") {
			continue
		}
		if !strings.Contains(envP, "=") {
			continue
		}
		envP = strings.ReplaceAll(envP, "\"", "")
		envPArr := strings.SplitN(envP, "=", 2)
		if len(envPArr) == 2 {
			os.Setenv(envPArr[0], envPArr[1])
		}
	}
}

func GetString(name, fallback string) string {
	env, ok := os.LookupEnv(name)
	if !ok {
		return fallback
	}
	return env
}
