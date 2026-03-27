package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"bytes"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ─── Tool 5: generate_instagram_carousel ──────────────────────────────────────

// Slide represents one slide in the carousel.
type Slide struct {
	Tag        string `json:"tag"`
	Heading    string `json:"heading"`
	Body       string `json:"body"`
	Background string `json:"background"` // "light", "dark", or "gradient"
	IsLast     bool
	Index      int
	Total      int
}

// CarouselConfig holds all brand + content data needed to render the HTML.
type CarouselConfig struct {
	BrandName    string
	Handle       string
	Primary      string // hex, e.g. "#6C63FF"
	Light        string
	Dark         string
	LightBG      string
	LightBorder  string
	DarkBG       string
	HeadingFont  string
	BodyFont     string
	Slides       []Slide
	TotalSlides  int
}

func registerGenerateInstagramCarousel(s *server.MCPServer) {
	tool := mcp.NewTool("generate_instagram_carousel",
		mcp.WithDescription(
			"Generate a self-contained, swipeable HTML Instagram carousel file (4:5 aspect ratio). "+
				"Returns the path to the HTML file. The file can be opened in a browser for preview and "+
				"exported to 1080×1350 PNGs using the companion Playwright export script.",
		),
		// ── Brand identity ──────────────────────────────────────────────────
		mcp.WithString("brandName",
			mcp.Required(),
			mcp.Description("Brand name shown on hero and CTA slides"),
		),
		mcp.WithString("handle",
			mcp.Required(),
			mcp.Description("Instagram handle (without @), e.g. 'mybrand'"),
		),
		mcp.WithString("primaryColor",
			mcp.Required(),
			mcp.Description("Primary brand color as a hex code, e.g. '#6C63FF'"),
		),
		mcp.WithString("fontStyle",
			mcp.Description(
				"Font pairing style. One of: editorial, modern, warm, technical, bold, classic, rounded. "+
					"Defaults to 'modern'.",
			),
		),
		mcp.WithString("tone",
			mcp.Description("Tone of the carousel, e.g. professional, casual, playful, bold, minimal"),
		),
		// ── Slide content ────────────────────────────────────────────────────
		mcp.WithString("sessionId",
			mcp.Required(),
			mcp.Description("Unique slug used as the output folder name, e.g. 'launch-week-0327'"),
		),
	)

	// slides is a JSON array of slide objects, defined outside mcp.WithString
	// because the SDK doesn't support array-of-object schema natively.
	tool.InputSchema.Properties["slides"] = map[string]any{
		"type":        "array",
		"description": "Ordered list of slides. Each slide has: tag (string), heading (string), body (string), background ('light'|'dark'|'gradient'). First slide is hero, last slide is CTA.",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tag":        map[string]any{"type": "string", "description": "Short uppercase category label, e.g. 'INTRODUCING'"},
				"heading":    map[string]any{"type": "string", "description": "Bold headline text"},
				"body":       map[string]any{"type": "string", "description": "Supporting body copy"},
				"background": map[string]any{"type": "string", "enum": []string{"light", "dark", "gradient"}, "description": "Slide background: light, dark, or gradient"},
			},
			"required": []string{"tag", "heading", "body", "background"},
		},
		"minItems": 3,
		"maxItems": 10,
	}
	tool.InputSchema.Required = append(tool.InputSchema.Required, "slides")

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// ── Required params ──────────────────────────────────────────────────
		brandName, err := req.RequireString("brandName")
		if err != nil {
			return nil, err
		}
		handle, err := req.RequireString("handle")
		if err != nil {
			return nil, err
		}
		primaryColor, err := req.RequireString("primaryColor")
		if err != nil {
			return nil, err
		}
		sessionId, err := req.RequireString("sessionId")
		if err != nil {
			return nil, err
		}

		// ── Optional params ──────────────────────────────────────────────────
		fontStyle := req.GetString("fontStyle", "modern")
		tone := req.GetString("tone", "")
		_ = tone // used in future prompting; stored for metadata

		// ── Decode slides ────────────────────────────────────────────────────
		slidesRaw, ok := req.GetArguments()["slides"]
		if !ok {
			return nil, fmt.Errorf("slides parameter is required")
		}
		slidesJSON, err := json.Marshal(slidesRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid slides: %w", err)
		}
		var rawSlides []struct {
			Tag        string `json:"tag"`
			Heading    string `json:"heading"`
			Body       string `json:"body"`
			Background string `json:"background"`
		}
		if err := json.Unmarshal(slidesJSON, &rawSlides); err != nil {
			return nil, fmt.Errorf("slides must be an array of objects: %w", err)
		}
		if len(rawSlides) < 3 || len(rawSlides) > 10 {
			return nil, fmt.Errorf("slides must have between 3 and 10 items, got %d", len(rawSlides))
		}

		// ── Build color palette from primary ─────────────────────────────────
		palette := deriveCarouselPalette(primaryColor)

		// ── Font pairing ─────────────────────────────────────────────────────
		headingFont, bodyFont := carouselFontPair(fontStyle)

		// ── Assemble slides ──────────────────────────────────────────────────
		slides := make([]Slide, len(rawSlides))
		for i, rs := range rawSlides {
			slides[i] = Slide{
				Tag:        rs.Tag,
				Heading:    rs.Heading,
				Body:       rs.Body,
				Background: rs.Background,
				IsLast:     i == len(rawSlides)-1,
				Index:      i,
				Total:      len(rawSlides),
			}
		}

		cfg := CarouselConfig{
			BrandName:   brandName,
			Handle:      handle,
			Primary:     palette.primary,
			Light:       palette.light,
			Dark:        palette.dark,
			LightBG:     palette.lightBG,
			LightBorder: palette.lightBorder,
			DarkBG:      palette.darkBG,
			HeadingFont: headingFont,
			BodyFont:    bodyFont,
			Slides:      slides,
			TotalSlides: len(slides),
		}

		// ── Render HTML ──────────────────────────────────────────────────────
		html, err := renderCarouselHTML(cfg)
		if err != nil {
			return nil, fmt.Errorf("render HTML: %w", err)
		}

		// ── Write to disk ────────────────────────────────────────────────────
		sessionDir := filepath.Join(outputDir, sessionId)
		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			return nil, err
		}

		ts := time.Now().Format("150405")
		htmlPath := filepath.Join(sessionDir, fmt.Sprintf("carousel_%s.html", ts))
		if err := os.WriteFile(htmlPath, []byte(html), 0644); err != nil {
			return nil, err
		}

		// ── Export script ────────────────────────────────────────────────────
		exportScript := buildPlaywrightExportScript(htmlPath, sessionDir, len(slides))
		exportScriptPath := filepath.Join(sessionDir, fmt.Sprintf("export_%s.py", ts))
		if err := os.WriteFile(exportScriptPath, []byte(exportScript), 0644); err != nil {
			return nil, err
		}

		out := map[string]any{
			"success":      true,
			"htmlFile":     htmlPath,
			"exportScript": exportScriptPath,
			"totalSlides":  len(slides),
			"instructions": "Open htmlFile in a browser to preview the carousel. Run exportScript with `python3 export_*.py` (requires playwright: `pip install playwright && playwright install chromium`) to export 1080×1350 PNGs.",
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

// ─── Palette derivation ───────────────────────────────────────────────────────

type carouselPalette struct {
	primary, light, dark, lightBG, lightBorder, darkBG string
}

// deriveCarouselPalette produces a 6-token palette from one hex color.
// It uses simple luminance shifts rather than a full color library so we
// have zero extra dependencies.
func deriveCarouselPalette(primary string) carouselPalette {
	p := carouselPalette{primary: primary}

	r, g, b := hexToRGBf(primary)

	// Lighten ~20% toward white
	lr := clamp01(r + 0.20)
	lg := clamp01(g + 0.20)
	lb := clamp01(b + 0.20)
	p.light = rgbfToHex(lr, lg, lb)

	// Darken ~30% toward black
	dr := clamp01(r - 0.30)
	dg := clamp01(g - 0.30)
	db := clamp01(b - 0.30)
	p.dark = rgbfToHex(dr, dg, db)

	// Light BG: very light tinted off-white (primary blended 8% into #F5F4F2)
	bgR := clampU8(0xF5 + int(r*255*0.06))
	bgG := clampU8(0xF4 + int(g*255*0.06))
	bgB := clampU8(0xF2 + int(b*255*0.06))
	p.lightBG = fmt.Sprintf("#%02X%02X%02X", bgR, bgG, bgB)

	// Light border: slightly darker than lightBG
	p.lightBorder = fmt.Sprintf("#%02X%02X%02X",
		clampU8(int(bgR)-18),
		clampU8(int(bgG)-18),
		clampU8(int(bgB)-18),
	)

	// Dark BG: near-black with subtle brand tint
	dbgR := clampU8(0x12 + int(r*255*0.08))
	dbgG := clampU8(0x10 + int(g*255*0.08))
	dbgB := clampU8(0x14 + int(b*255*0.08))
	p.darkBG = fmt.Sprintf("#%02X%02X%02X", dbgR, dbgG, dbgB)

	return p
}

func hexToRGBf(hex string) (float64, float64, float64) {
	c := hexToRGBA(hex)
	return float64(c.R) / 255.0, float64(c.G) / 255.0, float64(c.B) / 255.0
}

func rgbfToHex(r, g, b float64) string {
	return fmt.Sprintf("#%02X%02X%02X", uint8(r*255), uint8(g*255), uint8(b*255))
}

func clampU8(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// ─── Font pairs ───────────────────────────────────────────────────────────────

func carouselFontPair(style string) (heading, body string) {
	pairs := map[string][2]string{
		"editorial": {"Playfair Display", "DM Sans"},
		"modern":    {"Plus Jakarta Sans", "Plus Jakarta Sans"},
		"warm":      {"Lora", "Nunito Sans"},
		"technical": {"Space Grotesk", "Space Grotesk"},
		"bold":      {"Fraunces", "Outfit"},
		"classic":   {"Libre Baskerville", "Work Sans"},
		"rounded":   {"Bricolage Grotesque", "Bricolage Grotesque"},
	}
	if p, ok := pairs[style]; ok {
		return p[0], p[1]
	}
	return "Plus Jakarta Sans", "Plus Jakarta Sans"
}

// ─── HTML rendering ───────────────────────────────────────────────────────────

func renderCarouselHTML(cfg CarouselConfig) (string, error) {
	tmpl, err := template.New("carousel").Funcs(carouselFuncMap(cfg)).Parse(carouselHTMLTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func carouselFuncMap(cfg CarouselConfig) template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"pct": func(idx, total int) float64 {
			return float64(idx+1) / float64(total) * 100
		},
		"slideBG": func(bg string) string {
			switch bg {
			case "dark":
				return cfg.DarkBG
			case "gradient":
				return "linear-gradient(165deg, " + cfg.Dark + " 0%, " + cfg.Primary + " 50%, " + cfg.Light + " 100%)"
			default:
				return cfg.LightBG
			}
		},
		"isLight": func(bg string) bool {
			return bg == "light"
		},
		"tagColor": func(bg string) string {
			switch bg {
			case "dark":
				return cfg.Light
			case "gradient":
				return "rgba(255,255,255,0.6)"
			default:
				return cfg.Primary
			}
		},
		"headingColor": func(bg string) string {
			if bg == "light" {
				return cfg.DarkBG
			}
			return "#FFFFFF"
		},
		"bodyColor": func(bg string) string {
			if bg == "light" {
				return "#6B6560"
			}
			return "rgba(255,255,255,0.65)"
		},
		"trackColor": func(bg string) string {
			if bg == "light" {
				return "rgba(0,0,0,0.08)"
			}
			return "rgba(255,255,255,0.12)"
		},
		"fillColor": func(bg, primary string) string {
			if bg == "light" {
				return primary
			}
			return "#ffffff"
		},
		"counterColor": func(bg string) string {
			if bg == "light" {
				return "rgba(0,0,0,0.3)"
			}
			return "rgba(255,255,255,0.4)"
		},
		"arrowBG": func(bg string) string {
			if bg == "light" {
				return "rgba(0,0,0,0.06)"
			}
			return "rgba(255,255,255,0.08)"
		},
		"arrowStroke": func(bg string) string {
			if bg == "light" {
				return "rgba(0,0,0,0.25)"
			}
			return "rgba(255,255,255,0.35)"
		},
		"isBGGradient": func(bg string) bool {
			return bg == "gradient"
		},
		"googleFontsURL": func(h, b string) string {
			fonts := []string{h}
			if b != h {
				fonts = append(fonts, b)
			}
			var parts []string
			for _, f := range fonts {
				encoded := strings.ReplaceAll(f, " ", "+")
				parts = append(parts, "family="+encoded+":wght@300;400;600;700")
			}
			return "https://fonts.googleapis.com/css2?" + strings.Join(parts, "&") + "&display=swap"
		},
	}
}

// ─── Playwright export script ─────────────────────────────────────────────────

func buildPlaywrightExportScript(htmlPath, outputDir string, totalSlides int) string {
	return fmt.Sprintf(`#!/usr/bin/env python3
"""
Playwright export script — generated by snap-photo-mcp
Exports each slide as a 1080x1350 PNG ready for Instagram upload.

Requirements:
    pip install playwright
    playwright install chromium
"""
import asyncio
from pathlib import Path
from playwright.async_api import async_playwright

INPUT_HTML  = Path(%q)
OUTPUT_DIR  = Path(%q)
OUTPUT_DIR.mkdir(exist_ok=True)

TOTAL_SLIDES = %d

# Carousel is designed at 420px wide (4:5 → 525px tall).
# Scale factor: 1080 / 420 ≈ 2.5714  →  output is 1080x1350
VIEW_W = 420
VIEW_H = 525
SCALE  = 1080 / 420


async def export_slides():
    async with async_playwright() as p:
        browser = await p.chromium.launch()
        page = await browser.new_page(
            viewport={"width": VIEW_W, "height": VIEW_H},
            device_scale_factor=SCALE,
        )

        html_content = INPUT_HTML.read_text(encoding="utf-8")
        await page.set_content(html_content, wait_until="networkidle")
        await page.wait_for_timeout(3000)  # wait for Google Fonts

        # Strip IG frame chrome, expose only the slide viewport
        await page.evaluate("""() => {
            document.querySelectorAll('.ig-header,.ig-dots,.ig-actions,.ig-caption')
                     .forEach(el => el.style.display = 'none');

            const frame = document.querySelector('.ig-frame');
            frame.style.cssText =
                'width:420px;height:525px;max-width:none;border-radius:0;' +
                'box-shadow:none;overflow:hidden;margin:0;';

            const viewport = document.querySelector('.carousel-viewport');
            viewport.style.cssText =
                'width:420px;height:525px;aspect-ratio:unset;overflow:hidden;cursor:default;';

            document.body.style.cssText =
                'padding:0;margin:0;display:block;overflow:hidden;';
        }""")
        await page.wait_for_timeout(500)

        for i in range(TOTAL_SLIDES):
            await page.evaluate("""(idx) => {
                const track = document.querySelector('.carousel-track');
                track.style.transition = 'none';
                track.style.transform = 'translateX(' + (-idx * 420) + 'px)';
            }""", i)
            await page.wait_for_timeout(400)

            out_path = OUTPUT_DIR / f"slide_{i + 1:02d}.png"
            await page.screenshot(
                path=str(out_path),
                clip={"x": 0, "y": 0, "width": VIEW_W, "height": VIEW_H},
            )
            print(f"  ✓ slide {i + 1}/{TOTAL_SLIDES} → {out_path}")

        await browser.close()
        print("\\nDone! All slides exported to", OUTPUT_DIR)


asyncio.run(export_slides())
`, htmlPath, outputDir, totalSlides)
}

// ─── HTML template ────────────────────────────────────────────────────────────

const carouselHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"/>
<meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>{{.BrandName}} — Instagram Carousel</title>
<link rel="preconnect" href="https://fonts.googleapis.com"/>
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin/>
<link href="{{googleFontsURL .HeadingFont .BodyFont}}" rel="stylesheet"/>
<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

  body {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #1a1a2e;
    padding: 24px 16px 40px;
    font-family: '{{.BodyFont}}', sans-serif;
  }

  .serif  { font-family: '{{.HeadingFont}}', serif; }
  .sans   { font-family: '{{.BodyFont}}', sans-serif; }

  /* ── Instagram frame ─────────────────────────────── */
  .ig-frame {
    width: 420px;
    max-width: 100%;
    border-radius: 12px;
    box-shadow: 0 32px 80px rgba(0,0,0,0.6);
    overflow: hidden;
    background: #fff;
  }

  .ig-header {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    background: #fff;
    border-bottom: 1px solid #efefef;
  }

  .ig-avatar {
    width: 32px; height: 32px;
    border-radius: 50%;
    background: {{.Primary}};
    display: flex; align-items: center; justify-content: center;
    font-family: '{{.HeadingFont}}', serif;
    font-size: 14px; font-weight: 700; color: #fff;
    flex-shrink: 0;
  }

  .ig-handle { font-size: 13px; font-weight: 600; color: #262626; }
  .ig-sub    { font-size: 11px; color: #8E8E8E; }
  .ig-more   { margin-left: auto; font-size: 20px; color: #666; line-height:1; }

  /* ── Carousel viewport ───────────────────────────── */
  .carousel-viewport {
    width: 420px;
    aspect-ratio: 4/5;
    overflow: hidden;
    cursor: grab;
    position: relative;
    user-select: none;
  }

  .carousel-track {
    display: flex;
    height: 100%;
    transition: transform 0.35s cubic-bezier(0.25,0.46,0.45,0.94);
    will-change: transform;
  }

  /* ── Individual slide ────────────────────────────── */
  .slide {
    flex: 0 0 420px;
    height: 100%;
    position: relative;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
    padding: 0 36px 52px;
  }

  .slide.center { justify-content: center; }

  /* ── Progress bar ────────────────────────────────── */
  .progress-bar {
    position: absolute;
    bottom: 0; left: 0; right: 0;
    padding: 16px 28px 20px;
    z-index: 10;
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .progress-track {
    flex: 1;
    height: 3px;
    border-radius: 2px;
    overflow: hidden;
  }
  .progress-fill {
    height: 100%;
    border-radius: 2px;
  }
  .progress-counter {
    font-size: 11px;
    font-weight: 500;
    white-space: nowrap;
  }

  /* ── Swipe arrow ─────────────────────────────────── */
  .swipe-arrow {
    position: absolute;
    right: 0; top: 0; bottom: 0;
    width: 48px;
    z-index: 9;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  /* ── Tag label ───────────────────────────────────── */
  .tag-label {
    font-size: 10px;
    font-weight: 600;
    letter-spacing: 2px;
    text-transform: uppercase;
    margin-bottom: 16px;
    display: inline-block;
  }

  /* ── Dot indicators ──────────────────────────────── */
  .ig-dots {
    display: flex;
    justify-content: center;
    gap: 4px;
    padding: 10px 0;
    background: #fff;
  }
  .ig-dot {
    width: 6px; height: 6px;
    border-radius: 50%;
    background: #dbdbdb;
    transition: background 0.25s;
  }
  .ig-dot.active { background: {{.Primary}}; }

  /* ── Action bar ──────────────────────────────────── */
  .ig-actions {
    display: flex;
    align-items: center;
    padding: 8px 14px 4px;
    gap: 14px;
    background: #fff;
  }
  .ig-actions svg { color: #262626; }
  .ig-bookmark { margin-left: auto; }

  /* ── Caption ─────────────────────────────────────── */
  .ig-caption {
    padding: 4px 14px 14px;
    background: #fff;
    font-size: 13px;
    color: #262626;
    line-height: 1.5;
  }
  .ig-caption strong { font-weight: 700; }
  .ig-caption-time   { font-size: 10px; color: #8E8E8E; margin-top: 4px; }

  /* ── Logo lockup ─────────────────────────────────── */
  .logo-lockup {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 24px;
  }
  .logo-circle {
    width: 40px; height: 40px;
    border-radius: 50%;
    display: flex; align-items: center; justify-content: center;
    font-size: 18px; font-weight: 700;
  }
  .logo-name { font-size: 13px; font-weight: 600; letter-spacing: 0.5px; }

  /* ── CTA button ──────────────────────────────────── */
  .cta-btn {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 12px 28px;
    border-radius: 28px;
    font-size: 14px;
    font-weight: 600;
    margin-top: 24px;
    cursor: pointer;
  }
</style>
</head>
<body>

<div class="ig-frame">

  <!-- Header -->
  <div class="ig-header">
    <div class="ig-avatar">{{slice .BrandName 0 1}}</div>
    <div>
      <div class="ig-handle">{{.Handle}}</div>
      <div class="ig-sub">{{.BrandName}}</div>
    </div>
    <span class="ig-more">···</span>
  </div>

  <!-- Carousel viewport -->
  <div class="carousel-viewport" id="carouselViewport">
    <div class="carousel-track" id="carouselTrack">

      {{range $i, $slide := .Slides}}
      <!-- Slide {{add $i 1}}/{{$.TotalSlides}} -->
      <div class="slide{{if $slide.IsLast}} center{{end}}"
           style="background:{{if isBGGradient $slide.Background}}{{slideBG $slide.Background}}{{else}}{{slideBG $slide.Background}}{{end}};">

        {{/* Swipe arrow — hidden on last slide */}}
        {{if not $slide.IsLast}}
        <div class="swipe-arrow"
             style="background:linear-gradient(to right,transparent,{{arrowBG $slide.Background}});">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none">
            <path d="M9 6l6 6-6 6"
                  stroke="{{arrowStroke $slide.Background}}"
                  stroke-width="2.5"
                  stroke-linecap="round"
                  stroke-linejoin="round"/>
          </svg>
        </div>
        {{end}}

        <!-- Content -->
        {{if $slide.IsLast}}
        <!-- CTA / last slide -->
        <div class="logo-lockup">
          <div class="logo-circle" style="background:rgba(255,255,255,0.15);">
            <span class="serif" style="color:#fff;">{{slice $.BrandName 0 1}}</span>
          </div>
          <span class="logo-name sans" style="color:#fff;">{{$.BrandName}}</span>
        </div>
        <span class="tag-label sans" style="color:{{tagColor $slide.Background}};">{{$slide.Tag}}</span>
        <h2 class="serif" style="font-size:30px;font-weight:600;letter-spacing:-0.4px;line-height:1.1;color:#fff;margin-bottom:12px;">{{$slide.Heading}}</h2>
        <p class="sans" style="font-size:14px;line-height:1.55;color:rgba(255,255,255,0.65);">{{$slide.Body}}</p>
        <div class="cta-btn sans"
             style="background:rgba(255,255,255,0.15);color:#fff;border:1px solid rgba(255,255,255,0.25);">
          @{{$.Handle}}
        </div>
        {{else}}
        <!-- Regular slide -->
        <span class="tag-label sans" style="color:{{tagColor $slide.Background}};">{{$slide.Tag}}</span>
        <h2 class="serif" style="font-size:30px;font-weight:600;letter-spacing:-0.4px;line-height:1.1;color:{{headingColor $slide.Background}};margin-bottom:12px;">{{$slide.Heading}}</h2>
        <p class="sans" style="font-size:14px;line-height:1.55;color:{{bodyColor $slide.Background}};">{{$slide.Body}}</p>
        {{end}}

        <!-- Progress bar -->
        <div class="progress-bar">
          <div class="progress-track" style="background:{{trackColor $slide.Background}};">
            <div class="progress-fill"
                 style="width:{{printf "%.1f" (pct $i $.TotalSlides)}}%;background:{{fillColor $slide.Background $.Primary}};"></div>
          </div>
          <span class="progress-counter sans"
                style="color:{{counterColor $slide.Background}};">{{add $i 1}}/{{$.TotalSlides}}</span>
        </div>

      </div>
      {{end}}

    </div><!-- /.carousel-track -->
  </div><!-- /.carousel-viewport -->

  <!-- Dot indicators -->
  <div class="ig-dots" id="igDots">
    {{range $i, $slide := .Slides}}
    <div class="ig-dot{{if eq $i 0}} active{{end}}" data-idx="{{$i}}"></div>
    {{end}}
  </div>

  <!-- Action icons -->
  <div class="ig-actions">
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/>
    </svg>
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
    </svg>
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/>
      <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/>
      <line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>
    </svg>
    <svg class="ig-bookmark" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M19 21l-7-5-7 5V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z"/>
    </svg>
  </div>

  <!-- Caption -->
  <div class="ig-caption">
    <span><strong>{{.Handle}}</strong> Swipe through all {{.TotalSlides}} slides ✦</span>
    <div class="ig-caption-time">2 HOURS AGO</div>
  </div>

</div><!-- /.ig-frame -->

<script>
(function () {
  const track    = document.getElementById('carouselTrack');
  const viewport = document.getElementById('carouselViewport');
  const dots     = document.querySelectorAll('.ig-dot');
  const total    = {{.TotalSlides}};
  let current    = 0;
  let startX     = 0;
  let isDragging = false;

  function goTo(idx) {
    current = Math.max(0, Math.min(total - 1, idx));
    track.style.transform = 'translateX(' + (-current * 420) + 'px)';
    dots.forEach((d, i) => d.classList.toggle('active', i === current));
  }

  viewport.addEventListener('pointerdown', e => {
    isDragging = true;
    startX = e.clientX;
    viewport.setPointerCapture(e.pointerId);
    viewport.style.cursor = 'grabbing';
  });

  viewport.addEventListener('pointermove', e => {
    if (!isDragging) return;
    const diff = e.clientX - startX;
    track.style.transition = 'none';
    track.style.transform = 'translateX(' + (-current * 420 + diff) + 'px)';
  });

  viewport.addEventListener('pointerup', e => {
    if (!isDragging) return;
    isDragging = false;
    viewport.style.cursor = 'grab';
    track.style.transition = 'transform 0.35s cubic-bezier(0.25,0.46,0.45,0.94)';
    const diff = e.clientX - startX;
    if (diff < -40)       goTo(current + 1);
    else if (diff > 40)   goTo(current - 1);
    else                  goTo(current);
  });

  dots.forEach((d, i) => d.addEventListener('click', () => goTo(i)));
})();
</script>
</body>
</html>`
