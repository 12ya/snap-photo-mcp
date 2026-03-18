# tiktok-image-gen (Go)

MCP server that generates TikTok photo carousels using the Gemini image API, applies text overlays, and posts directly to TikTok. Single binary, no runtime dependencies.

## Requirements

- Go 1.21+
- A Gemini API key ([get one here](https://aistudio.google.com/app/apikey))
- TikTok Developer credentials (only needed for `post_to_tiktok`)

## Setup

```bash
git clone https://github.com/12ya/social-media-mcp/
cd social-media-mcp/go
go build .
```

This produces a single `social-media-mcp` binary.

## MCP config

Create a `.mcp.json` at the repo root:

```json
{
  "mcpServers": {
    "tiktok-image-gen": {
      "command": "./social-media-mcp",
      "env": {
        "GEMINI_API_KEY": "your_gemini_key",
        "IMAGE_OUTPUT_DIR": "/Users/you/Downloads"
      }
    }
  }
}
```

The binary is launched with `cwd` set to the repo root, so `core_pics/` is found automatically.

Restart your MCP client after editing the config.

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `GEMINI_API_KEY` | Yes (for image gen) | — | Gemini API key |
| `IMAGE_OUTPUT_DIR` | No | `~/Downloads` | Where generated images are saved |
| `IMAGE_QUALITY` | No | `fast` | `fast` (flash model) or `quality` (pro model) |

> **Note:** The binary must be launched from the repo root (`.mcp.json` does this automatically) so that `core_pics/` is found correctly.

## Tools

### `generate_slideshow`
Generates a 6-slide TikTok photo carousel via Gemini. Returns paths to all 6 PNGs.

```
sessionId          string   Unique slug for this post, e.g. "coding-morning-0317"
hookText           string   Bold hook text overlaid on slide 1
lockedArchitecture string   Fixed scene description pasted into every prompt
styleVariants      string[] 6 style variations, one per slide
title              string?  Post title (saved to caption.txt)
caption            string?  Full caption + hashtags (saved to caption.txt)
```

### `generate_single_image`
Generates one image from a raw prompt. Useful for testing prompts.

```
prompt    string  Full image generation prompt
filename  string  Output filename (no extension)
hookText  string? If provided, applies text overlay
```

### `add_text_overlay`
Applies hook text to an existing PNG without regenerating it.

```
imagePath  string  Absolute path to source PNG
hookText   string  Text to overlay
outputPath string? Destination path (defaults to overwriting source)
```

### `brand_core_pic`
Applies a brand color grade and caption band to a real app screenshot from `core_pics/`.

```
picName      string  Filename in core_pics/ — pass "list" to see all available
text         string? Text to overlay (Snapchat-style center band)
caption      string? Full TikTok caption, saved as .txt alongside the image
colorGrade   string? sky-blue | sage-green | soft-violet | warm-amber | muted-coral | lavender | teal | golden
gradeOpacity number? Color grade strength 0–1 (default 0.25)
outputPath   string? Destination path
```

### `tiktok_exchange_code`
One-time OAuth setup. Exchanges a TikTok authorization code for access + refresh tokens and writes them to `.env`.

```
code          string  Authorization code from TikTok OAuth callback (?code=...)
redirect_uri  string  Must match the redirect_uri used when requesting the code
code_verifier string? PKCE code verifier (mobile/desktop flows only)
```

Requires `TIKTOK_CLIENT_KEY` and `TIKTOK_CLIENT_SECRET` in `.env` before running.

### `post_to_tiktok`
Uploads a photo carousel to TikTok via the Content Posting API. Post lands in your TikTok inbox as a draft.

```
imagePaths   string[] Absolute paths to PNGs in carousel order (2–35 images)
title        string?  Post title (max 150 chars)
description  string?  Caption (max 2200 chars)
privacyLevel string?  PUBLIC_TO_EVERYONE | MUTUAL_FOLLOW_FRIENDS | FOLLOWER_OF_CREATOR | SELF_ONLY (default)
coverIndex   number?  Cover image index, 0-based (default 0)
```

Requires `TIKTOK_CLIENT_KEY`, `TIKTOK_CLIENT_SECRET`, and `TIKTOK_REFRESH_TOKEN` in `.env`.

## TikTok auth setup

1. Create an app in the [TikTok Developer Portal](https://developers.tiktok.com)
2. Add `TIKTOK_CLIENT_KEY` and `TIKTOK_CLIENT_SECRET` to `.env`
3. Send your user through TikTok's OAuth flow, copy the `?code=` from the redirect URL
4. Call `tiktok_exchange_code` — tokens are written to `.env` automatically
5. Restart the MCP server

## Usage examples

These are prompts you give Claude (with the MCP server connected) — not shell commands.

---

### Generate 6 branded app screenshots

> "Generate 6 core brand images"

Claude will call `brand_core_pic` for each of the 6 app screenshots in `core_pics/`, applying the correct slide color per the FocusHero palette (sky-blue → sage-green → soft-violet → warm-amber → muted-coral), with value text on each slide. All images land in a single timestamped folder, e.g. `~/Downloads/branded-20260318-1402/`.

---

### Generate a themed AI slideshow

> "Generate a piano mastery slideshow"

Claude will call `generate_slideshow` with a piano-themed `lockedArchitecture` (close-up of keys, sheet music, ambient candlelight), a hook on slide 1, and 5 style variants for slides 2–6. Gemini generates all 6 images in parallel. Output lands in `~/Downloads/piano-mastery-{date}/`.

Other examples:
- `"Generate a coding deep-work slideshow for morning"`
- `"Generate a reading streak slideshow"`
- `"Generate a design sprint slideshow with warm tones"`

---

### Brand a single screenshot

> "Brand the habit tracker screenshot with sage green and the text 'Build streaks across everything you're working toward.'"

Claude calls `brand_core_pic` once. Pass `picName: "list"` first if you're unsure of the exact filename.

---

### Post a carousel to TikTok

> "Post the branded images in ~/Downloads/branded-20260318-1402/ to TikTok as a draft"

Claude calls `post_to_tiktok` with the image paths in carousel order. The post lands in your TikTok inbox as a **Self Only** draft for review before publishing.

---

## Quick test (no API key needed)

```bash
cd /path/to/social-media-mcp

echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"brand_core_pic","arguments":{"picName":"todos.PNG","text":"Track every session.","colorGrade":"sky-blue"}}}' \
  | CORE_PICS_DIR=/path/to/social-media-mcp/core_pics \
    ./go/social-media-mcp
```

Output is saved to `~/Downloads/branded-{timestamp}/todos_branded.png`.
