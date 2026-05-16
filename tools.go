package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

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
			wg.Go(func() {
				defer wg.Done()
				prompt := buildPrompt(style)
				var hookPtr *string
				if i == 0 {
					hookPtr = &hookText
				}
				results[i], errs[i] = generateSingle(prompt, hookPtr, sessionDir, i+1, "")
			})
		}
		wg.Wait()

		failures := map[string]string{}
		for i, err := range errs {
			if err != nil {
				failures[fmt.Sprintf("slide_%d", i+1)] = err.Error()
			}
		}

		out := map[string]any{
			"success":   len(failures) == 0,
			"sessionId": sessionId,
			"files":     results,
		}
		if len(failures) > 0 {
			out["failures"] = failures
		}
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
		mcp.WithString("viralCaption", mcp.Description("Viral title + description saved once as caption.txt in the batch folder (shared across all images in the batch)")),
		mcp.WithString("colorGrade", mcp.Description("Brand color tint: sky-blue, sage-green, soft-violet, warm-amber, muted-coral, lavender, teal, golden")),
		mcp.WithNumber("gradeOpacity", mcp.Description("Color grade strength 0-1 (default 0.25)")),
		mcp.WithString("batchID", mcp.Description("Shared folder name suffix for a batch (e.g. \"launch-week\"). All calls with the same batchID land in the same folder. Defaults to current timestamp.")),
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

		if picName == "list" || !slices.Contains(available, picName) {
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
		baseName := regexp.MustCompile(`(\.[^.]+)$`).ReplaceAllString(picName, "_branded.png")
		batchID := req.GetString("batchID", "")
		if batchID == "" {
			batchID = time.Now().Format("20060102_1504")
		}
		folder := filepath.Join(outputDir, "branded_"+batchID)
		dest := filepath.Join(folder, baseName)
		if err := os.MkdirAll(folder, 0755); err != nil {
			return nil, err
		}

		text := req.GetString("text", "")
		viralCaption := req.GetString("viralCaption", "")
		colorGrade := req.GetString("colorGrade", "")
		gradeOpacity := req.GetFloat("gradeOpacity", 0.25)

		if err := applyBrandedOverlay(src, dest, text, colorGrade, gradeOpacity); err != nil {
			return nil, err
		}

		var lines []string
		lines = append(lines, fmt.Sprintf("Image → %s", dest))

		if viralCaption != "" {
			captionFile := filepath.Join(folder, "caption.txt")
			if err := os.WriteFile(captionFile, []byte(viralCaption), 0644); err != nil {
				return nil, err
			}
			lines = append(lines, fmt.Sprintf("Caption → %s", captionFile))
		}

		return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
	})
}

// ─── Tool 5: generate_brand_story_pics ────────────────────────────────────────

func registerGenerateBrandStoryPics(s *server.MCPServer) {
	tool := mcp.NewTool("generate_brand_story_pics",
		mcp.WithDescription("Generate a 6-image FocusHero brand screenshot carousel with one continuous story across all slides"),
		mcp.WithString("sessionId", mcp.Description("Shared folder name suffix, e.g. 'coding-story-0317'. Defaults to current timestamp.")),
		mcp.WithString("viralCaption", mcp.Description("Viral title + description saved as caption.txt in the batch folder")),
		mcp.WithNumber("gradeOpacity", mcp.Description("Color grade strength 0-1 (default 0.25)")),
	)
	tool.InputSchema.Properties["storyTexts"] = map[string]any{
		"type":        "array",
		"description": "Exactly 6 short story beats. Each beat is overlaid on the matching slide and should read as one continuous story.",
		"items":       map[string]any{"type": "string"},
		"minItems":    6,
		"maxItems":    6,
	}
	tool.InputSchema.Properties["picNames"] = map[string]any{
		"type":        "array",
		"description": "Optional 6 filenames from core_pics/. Defaults to a balanced FocusHero app journey.",
		"items":       map[string]any{"type": "string"},
		"minItems":    6,
		"maxItems":    6,
	}
	tool.InputSchema.Required = append(tool.InputSchema.Required, "storyTexts")

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		storyTexts, err := req.RequireStringSlice("storyTexts")
		if err != nil {
			return nil, err
		}
		if len(storyTexts) != 6 {
			return nil, fmt.Errorf("storyTexts must have exactly 6 items")
		}

		picNames, err := req.RequireStringSlice("picNames")
		if err != nil || len(picNames) == 0 {
			picNames = defaultStoryPicNames()
		}
		if len(picNames) != 6 {
			return nil, fmt.Errorf("picNames must have exactly 6 items when provided")
		}

		available, err := availableCorePics()
		if err != nil {
			return nil, err
		}
		for _, picName := range picNames {
			if !slices.Contains(available, picName) {
				return nil, fmt.Errorf("picName %q not found in core_pics/. Available: %s", picName, strings.Join(available, ", "))
			}
		}

		sessionId := req.GetString("sessionId", "")
		if sessionId == "" {
			sessionId = time.Now().Format("20060102_1504")
		}
		folder := filepath.Join(outputDir, "brand_story_"+sessionId)
		if err := os.MkdirAll(folder, 0755); err != nil {
			return nil, err
		}

		gradeOpacity := req.GetFloat("gradeOpacity", 0.25)
		colorGrades := []string{"sky-blue", "sage-green", "soft-violet", "warm-amber", "muted-coral", "teal"}
		files := make([]string, 6)

		for i, picName := range picNames {
			src := filepath.Join(corePicsDir, picName)
			dest := filepath.Join(folder, fmt.Sprintf("slide_%d_%s", i+1, storySafeName(picName)))
			if err := applyBrandedOverlay(src, dest, storyTexts[i], colorGrades[i], gradeOpacity); err != nil {
				return nil, err
			}
			files[i] = dest
		}

		out := map[string]any{
			"success":    true,
			"sessionId":  sessionId,
			"folder":     folder,
			"files":      files,
			"storyTexts": storyTexts,
			"picNames":   picNames,
		}

		if viralCaption := req.GetString("viralCaption", ""); viralCaption != "" {
			captionFile := filepath.Join(folder, "caption.txt")
			if err := os.WriteFile(captionFile, []byte(viralCaption), 0644); err != nil {
				return nil, err
			}
			out["captionFile"] = captionFile
		}

		b, _ := json.MarshalIndent(out, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func availableCorePics() ([]string, error) {
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
	return available, nil
}

func defaultStoryPicNames() []string {
	return []string{
		"Main (started) dark.png",
		"Todos.png",
		"Habit Tracker.png",
		"Analytics - building (dark).png",
		"Hero journey Card (dark).png",
		"Hero journey Levels (dark).png",
	}
}

func storySafeName(picName string) string {
	base := regexp.MustCompile(`\.[^.]+$`).ReplaceAllString(picName, "")
	base = strings.ToLower(base)
	base = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(base, "_")
	base = strings.Trim(base, "_")
	if base == "" {
		return "brand_story.png"
	}
	return base + ".png"
}
