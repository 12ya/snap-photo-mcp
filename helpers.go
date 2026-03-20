package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func buildPrompt(style string) string {
	return fmt.Sprintf("iPhone photo. %s Portrait orientation. Realistic lighting, natural phone camera quality.",
		strings.TrimSpace(style))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func homeDir(sub string) string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, sub)
}

func mustAbs(rel string) string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, rel)
}

func filterEmpty(ss []string) []string {
	var out []string
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
