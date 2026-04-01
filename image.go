package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fogleman/gg"
)

// ─── Gemini image generation ──────────────────────────────────────────────────

type geminiRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig geminiGenConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	ResponseModalities []string          `json:"responseModalities"`
	ImageConfig        geminiImageConfig `json:"imageConfig"`
}

type geminiImageConfig struct {
	AspectRatio string `json:"aspectRatio"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text       string `json:"text"`
				InlineData *struct {
					MimeType string `json:"mimeType"`
					Data     string `json:"data"`
				} `json:"inlineData"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func geminiGenerateImage(prompt string) ([]byte, error) {
	model := geminiModels[imageQuality]
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, geminiKey)

	body, _ := json.Marshal(geminiRequest{
		Contents: []geminiContent{{Parts: []geminiPart{{Text: prompt}}}},
		GenerationConfig: geminiGenConfig{
			ResponseModalities: []string{"IMAGE"},
			ImageConfig:        geminiImageConfig{AspectRatio: "9:16"},
		},
	})

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var result geminiResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("gemini parse failed: %w", err)
	}

	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("gemini returned no candidates: %s", string(raw))
	}

	for _, part := range result.Candidates[0].Content.Parts {
		if part.InlineData != nil && part.InlineData.Data != "" {
			return base64.StdEncoding.DecodeString(part.InlineData.Data)
		}
	}

	// Extract any text error message from response
	for _, part := range result.Candidates[0].Content.Parts {
		if part.Text != "" {
			return nil, fmt.Errorf("gemini returned no image: %s", part.Text)
		}
	}
	return nil, fmt.Errorf("gemini returned no image data")
}

// generateSingle generates one image, writes it to dir, and returns the final path.
// hookText: if non-nil, applies text overlay. nameOverride: if non-empty, uses that base name.
func generateSingle(prompt string, hookText *string, dir string, index int, nameOverride string) (string, error) {
	imgBytes, err := geminiGenerateImage(prompt)
	if err != nil {
		return "", err
	}

	name := nameOverride
	if name == "" {
		name = fmt.Sprintf("slide_%d", index)
	}

	finalPath := filepath.Join(dir, name+".png")

	if hookText != nil && *hookText != "" {
		// Write raw temporarily, overlay, delete raw
		rawPath := filepath.Join(dir, name+"_raw.png")
		if err := os.WriteFile(rawPath, imgBytes, 0644); err != nil {
			return "", err
		}
		if err := applyTextOverlay(rawPath, *hookText, finalPath); err != nil {
			os.Remove(rawPath)
			return "", err
		}
		os.Remove(rawPath)
	} else {
		if err := os.WriteFile(finalPath, imgBytes, 0644); err != nil {
			return "", err
		}
	}

	return finalPath, nil
}

// ─── Text overlay ─────────────────────────────────────────────────────────────
// Matches TS rules:
//   - Font bold, 6.5% of image height (~100px on 1536h)
//   - Lower third, above bottom 12% (TikTok UI zone)
//   - White text with black shadow, max 82% width, word-wrapped

func applyTextOverlay(srcPath, text, destPath string) error {
	src, err := gg.LoadImage(srcPath)
	if err != nil {
		return fmt.Errorf("load image: %w", err)
	}
	bounds := src.Bounds()
	W := float64(bounds.Dx())
	H := float64(bounds.Dy())

	dc := gg.NewContext(int(W), int(H))
	dc.DrawImage(src, 0, 0)

	fontSize := H * 0.065
	if err := loadBoldFont(dc, fontSize); err != nil {
		return fmt.Errorf("load font: %w", err)
	}

	maxLineWidth := W * 0.82
	lineHeight := fontSize * 1.25

	lines := wordWrap(dc, text, maxLineWidth)

	blockHeight := float64(len(lines)) * lineHeight
	bottomSafe := H * 0.12
	startY := H - bottomSafe - blockHeight - H*0.06

	shadowOffset := fontSize * 0.06
	if shadowOffset < 1 {
		shadowOffset = 1
	}

	for i, line := range lines {
		lineW, _ := dc.MeasureString(line)
		x := (W - lineW) / 2
		y := startY + float64(i)*lineHeight + fontSize // offset for baseline

		// Shadow pass
		dc.SetRGBA(0, 0, 0, 0.85)
		for dx := -shadowOffset; dx <= shadowOffset; dx += shadowOffset {
			for dy := -shadowOffset; dy <= shadowOffset; dy += shadowOffset {
				dc.DrawString(line, x+dx, y+dy)
			}
		}

		// White text
		dc.SetHexColor("#FFFFFF")
		dc.DrawString(line, x, y)
	}

	return dc.SavePNG(destPath)
}

// ─── Brand overlay ────────────────────────────────────────────────────────────
// Applies multiply color grade + Snapchat-style center caption band.

func applyBrandedOverlay(srcPath, destPath, text, colorGrade string, gradeOpacity float64) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	src, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	// Resolve brand color early — used for both multiply blend and background glow
	var working image.Image = src
	var brandHex string
	if colorGrade != "" {
		if hex, ok := brandGrades[colorGrade]; ok {
			brandHex = hex
			working = multiplyBlend(working, hex, gradeOpacity)
		}
	}

	bounds := working.Bounds()
	W := float64(bounds.Dx())
	H := float64(bounds.Dy())

	dc := gg.NewContext(int(W), int(H))

	// Radial gradient background — brand color glow at center fading to near-black
	if brandHex != "" {
		bc := hexToRGBA(brandHex)
		grad := gg.NewRadialGradient(W/2, H/2, 0, W/2, H/2, H*0.65)
		grad.AddColorStop(0, color.RGBA{
			R: uint8(float64(bc.R) * 0.22),
			G: uint8(float64(bc.G) * 0.22),
			B: uint8(float64(bc.B) * 0.22),
			A: 255,
		})
		grad.AddColorStop(1, color.RGBA{14, 14, 16, 255})
		dc.SetFillStyle(grad)
		dc.DrawRectangle(0, 0, W, H)
		dc.Fill()
	} else {
		dc.SetHexColor("#0E0E10")
		dc.Clear()
	}

	// Scale down the screenshot and center it so the whole phone is visible
	const phoneScale = 0.55
	dc.Push()
	dc.Translate(W/2, H/2)
	dc.Scale(phoneScale, phoneScale)
	dc.DrawImageAnchored(working, 0, 0, 0.5, 0.5)
	dc.Pop()

	// Snapchat-style caption band — font is fixed, band grows to fit text
	if text != "" {
		fontSize := H * 0.05 * 0.65 // same starting size as before
		if fontSize < 12 {
			fontSize = 12
		}
		if err := loadBoldFont(dc, fontSize); err != nil {
			return err
		}

		// Wrap text so no line exceeds 90% of width
		lines := dc.WordWrap(text, W*0.9)
		lineH := fontSize * 1.3
		bandH := lineH*float64(len(lines)) + fontSize*0.6 // padding top+bottom
		bandY := H/2 - bandH/2

		// Band background
		dc.SetRGBA(0, 0, 0, 0.72)
		dc.DrawRectangle(0, bandY, W, bandH)
		dc.Fill()

		// Draw each line centered in the band
		dc.SetHexColor("#FFFFFF")
		for i, line := range lines {
			// ay=0: gg draws from the baseline directly; place baseline at 75% of slot
			// height so the visual glyph center lands at the slot midpoint
			y := bandY + fontSize*0.3 + float64(i)*lineH + lineH*0.75
			dc.DrawStringAnchored(line, W/2, y, 0.5, 0)
		}
	}

	outF, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outF.Close()
	return png.Encode(outF, dc.Image())
}

// ─── Multiply blend ───────────────────────────────────────────────────────────
// Implements CSS/Canvas `multiply` blend mode with opacity:
// result = src * (src * color / 255) blended at `opacity`

func multiplyBlend(img image.Image, hexColor string, opacity float64) image.Image {
	col := hexToRGBA(hexColor)
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)

	cr := float64(col.R) / 255.0
	cg := float64(col.G) / 255.0
	cb := float64(col.B) / 255.0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			sr, sg, sb, sa := img.At(x, y).RGBA()
			// RGBA() returns [0, 65535]; normalize to [0, 1]
			r := float64(sr) / 65535.0
			g := float64(sg) / 65535.0
			b := float64(sb) / 65535.0
			a := float64(sa) / 65535.0

			// multiply blend then interpolate with opacity
			mr := r*(1-opacity) + (r*cr)*opacity
			mg := g*(1-opacity) + (g*cg)*opacity
			mb := b*(1-opacity) + (b*cb)*opacity

			dst.Set(x, y, color.RGBA{
				R: uint8(clamp01(mr) * 255),
				G: uint8(clamp01(mg) * 255),
				B: uint8(clamp01(mb) * 255),
				A: uint8(clamp01(a) * 255),
			})
		}
	}
	return dst
}

// ─── Font loading ─────────────────────────────────────────────────────────────
// Tries common macOS / Linux bold font paths. gg requires a .ttf file path.

var boldFontPaths = []string{
	"/System/Library/Fonts/Supplemental/Arial Bold.ttf",
	"/Library/Fonts/Arial Bold.ttf",
	"/System/Library/Fonts/Supplemental/Arial.ttf",
	"/Library/Fonts/Arial.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/truetype/freefont/FreeSansBold.ttf",
}

func loadBoldFont(dc *gg.Context, size float64) error {
	for _, p := range boldFontPaths {
		if _, err := os.Stat(p); err == nil {
			return dc.LoadFontFace(p, size)
		}
	}
	return fmt.Errorf("no bold font found; install Arial or set a font path in boldFontPaths")
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func wordWrap(dc *gg.Context, text string, maxWidth float64) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		test := current + " " + word
		w, _ := dc.MeasureString(test)
		if w <= maxWidth {
			current = test
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	return append(lines, current)
}

func hexToRGBA(hex string) color.RGBA {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return color.RGBA{A: 255}
	}
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
