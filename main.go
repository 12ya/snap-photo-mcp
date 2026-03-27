package main

import (
	"log"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

var (
	outputDir    = getEnv("IMAGE_OUTPUT_DIR", homeDir("Downloads"))
	geminiKey    = os.Getenv("GEMINI_API_KEY")
	imageQuality = getEnv("IMAGE_QUALITY", "fast")
	corePicsDir  = getEnv("CORE_PICS_DIR", mustAbs("core_pics"))
	envPath      = mustAbs(".env")
)

var geminiModels = map[string]string{
	"fast":    "gemini-3.1-flash-image-preview",
	"quality": "gemini-3-pro-image-preview",
}

var brandGrades = map[string]string{
	"sky-blue":    "#85BDEB",
	"sage-green":  "#8CC799",
	"soft-violet": "#AD85D9",
	"warm-amber":  "#EBC285",
	"muted-coral": "#EB9E85",
	"lavender":    "#B89EE6",
	"teal":        "#73C7BD",
	"golden":      "#E6D680",
}

const tiktokAPI = "https://open.tiktokapis.com"

func main() {
	s := server.NewMCPServer("tiktok-image-gen", "1.0.0")

	registerGenerateSlideshow(s)
	registerAddTextOverlay(s)
	registerGenerateSingleImage(s)
	registerBrandCorePic(s)
	registerGenerateInstagramCarousel(s)
	// registerTikTokExchangeCode(s)
	// registerPostToTikTok(s)

	// Serve outputs on localhost:8080
	go func() {
		fs := http.FileServer(http.Dir(outputDir))
		http.Handle("/", fs)
		log.Println("Preview server running on http://localhost:8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}
