package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ─── Config ───────────────────────────────────────────────────────────────────

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

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	s := server.NewMCPServer("tiktok-image-gen", "1.0.0")

	registerGenerateSlideshow(s)
	registerAddTextOverlay(s)
	registerGenerateSingleImage(s)
	registerBrandCorePic(s)
	// registerTikTokExchangeCode(s)
	// registerPostToTikTok(s)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}

// ─── Tool 1: generate_slideshow ───────────────────────────────────────────────

func registerGenerateSlideshow(s *server.MCPServer) {
	tool := mcp.NewTool("generate_slideshow",
		mcp.WithDescription("Generate a 6-slide TikTok photo carousel"),
		mcp.WithString("sessionId", mcp.Required(), mcp.Description("Unique slug, e.g. 'coding-morning-0317'")),
		mcp.WithString("hookText", mcp.Required(), mcp.Description("Bold hook overlaid on slide 1")),
		mcp.WithString("title", mcp.Description("Viral TikTok title for the post")),
		mcp.WithString("caption", mcp.Description("Full TikTok caption including hashtags")),
	)
	tool.InputSchema.Properties["styleVariants"] = map[string]any{
		"type":        "array",
		"description": "6 style-only additions, one per slide",
		"items":       map[string]any{"type": "string"},
		"minItems":    6,
		"maxItems":    6,
	}
	tool.InputSchema.Required = append(tool.InputSchema.Required, "styleVariants")

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessionId, err := req.RequireString("sessionId")
		if err != nil {
			return nil, err
		}
		hookText, err := req.RequireString("hookText")
		if err != nil {
			return nil, err
		}
		styleVariants, err := req.RequireStringSlice("styleVariants")
		if err != nil {
			return nil, err
		}
		if len(styleVariants) != 6 {
			return nil, fmt.Errorf("styleVariants must have exactly 6 items")
		}
		title := req.GetString("title", "")
		caption := req.GetString("caption", "")

		sessionDir := filepath.Join(outputDir, sessionId)
		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			return nil, err
		}

		results := make([]string, 6)
		errs := make([]error, 6)
		var wg sync.WaitGroup
		for i, style := range styleVariants {
			wg.Add(1)
			go func(i int, style string) {
				defer wg.Done()
				prompt := buildPrompt(style)
				var hookPtr *string
				if i == 0 {
					hookPtr = &hookText
				}
				results[i], errs[i] = generateSingle(prompt, hookPtr, sessionDir, i+1, "")
			}(i, style)
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				return nil, fmt.Errorf("slide %d: %w", i+1, err)
			}
		}

		out := map[string]any{"success": true, "sessionId": sessionId, "files": results}
		if title != "" || caption != "" {
			lines := strings.Join(filterEmpty([]string{title, caption}), "\n\n")
			captionFile := filepath.Join(sessionDir, "caption.txt")
			if err := os.WriteFile(captionFile, []byte(lines), 0644); err != nil {
				return nil, err
			}
			out["captionFile"] = captionFile
		}

		b, _ := json.MarshalIndent(out, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

// ─── Tool 2: add_text_overlay ─────────────────────────────────────────────────

func registerAddTextOverlay(s *server.MCPServer) {
	tool := mcp.NewTool("add_text_overlay",
		mcp.WithDescription("Apply hook text to an already-generated image file"),
		mcp.WithString("imagePath", mcp.Required(), mcp.Description("Absolute path to source PNG (1024x1536)")),
		mcp.WithString("hookText", mcp.Required(), mcp.Description("Text to overlay")),
		mcp.WithString("outputPath", mcp.Description("Where to save result (defaults to overwrite)")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		imagePath, err := req.RequireString("imagePath")
		if err != nil {
			return nil, err
		}
		hookText, err := req.RequireString("hookText")
		if err != nil {
			return nil, err
		}
		dest := req.GetString("outputPath", imagePath)

		if err := applyTextOverlay(imagePath, hookText, dest); err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(fmt.Sprintf("Overlay applied → %s", dest)), nil
	})
}

// ─── Tool 3: generate_single_image ────────────────────────────────────────────

func registerGenerateSingleImage(s *server.MCPServer) {
	tool := mcp.NewTool("generate_single_image",
		mcp.WithDescription("Generate one image from a raw prompt"),
		mcp.WithString("prompt", mcp.Required()),
		mcp.WithString("filename", mcp.Required(), mcp.Description("Filename without extension")),
		mcp.WithString("hookText", mcp.Description("If provided, overlay text is applied")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		prompt, err := req.RequireString("prompt")
		if err != nil {
			return nil, err
		}
		filename, err := req.RequireString("filename")
		if err != nil {
			return nil, err
		}
		hookText := req.GetString("hookText", "")

		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return nil, err
		}

		var hookPtr *string
		if hookText != "" {
			hookPtr = &hookText
		}

		imgPath, err := generateSingle(prompt, hookPtr, outputDir, 1, filename)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(fmt.Sprintf("Saved → %s", imgPath)), nil
	})
}

// ─── Tool 4: brand_core_pic ───────────────────────────────────────────────────

func registerBrandCorePic(s *server.MCPServer) {
	tool := mcp.NewTool("brand_core_pic",
		mcp.WithDescription("Composite brand styling onto a real FocusHero app screenshot"),
		mcp.WithString("picName", mcp.Required(), mcp.Description(`Filename in core_pics/ (e.g. "todos.PNG"). Pass "list" to see all.`)),
		mcp.WithString("text", mcp.Description("Text to overlay on the image")),
		mcp.WithString("caption", mcp.Description("Full TikTok caption saved as a .txt alongside the image")),
		mcp.WithString("colorGrade", mcp.Description("Brand color tint: sky-blue, sage-green, soft-violet, warm-amber, muted-coral, lavender, teal, golden")),
		mcp.WithNumber("gradeOpacity", mcp.Description("Color grade strength 0-1 (default 0.25)")),
		mcp.WithString("outputPath", mcp.Description("Destination path")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		picName, err := req.RequireString("picName")
		if err != nil {
			return nil, err
		}

		entries, err := os.ReadDir(corePicsDir)
		if err != nil {
			return nil, fmt.Errorf("core_pics dir not found: %w", err)
		}

		var available []string
		for _, e := range entries {
			if !e.IsDir() {
				name := e.Name()
				lower := strings.ToLower(name)
				if strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
					available = append(available, name)
				}
			}
		}

		if picName == "list" || !contains(available, picName) {
			if len(available) == 0 {
				return mcp.NewToolResultText("No image files found in core_pics/"), nil
			}
			var sb strings.Builder
			sb.WriteString("Available pics in core_pics/:\n")
			for _, f := range available {
				sb.WriteString("  • " + f + "\n")
			}
			return mcp.NewToolResultText(sb.String()), nil
		}

		src := filepath.Join(corePicsDir, picName)
		dest := req.GetString("outputPath", "")
		if dest == "" {
			baseName := regexp.MustCompile(`(\.[^.]+)$`).ReplaceAllString(picName, "_branded.png")
			dest = filepath.Join(outputDir, "branded-"+time.Now().Format("20060102-1504"), baseName)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return nil, err
		}

		text := req.GetString("text", "")
		caption := req.GetString("caption", "")
		colorGrade := req.GetString("colorGrade", "")
		gradeOpacity := req.GetFloat("gradeOpacity", 0.25)

		if err := applyBrandedOverlay(src, dest, text, colorGrade, gradeOpacity); err != nil {
			return nil, err
		}

		var lines []string
		lines = append(lines, fmt.Sprintf("Image   → %s", dest))

		if caption != "" {
			captionFile := regexp.MustCompile(`\.png$`).ReplaceAllString(dest, ".txt")
			if err := os.WriteFile(captionFile, []byte(caption), 0644); err != nil {
				return nil, err
			}
			lines = append(lines, fmt.Sprintf("Caption → %s", captionFile))
		}

		return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

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

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
