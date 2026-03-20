# TikTok Slideshow Generator

## What this does
Generates 6-slide TikTok photo carousels for a mobile app using the `generate_slideshow` MCP tool.
Each slideshow: 6 portrait images (1024×1536), hook text on slide 1, value text on slides 2–6.

---

## App context
**YourApp** — brief one-liner describing what the app does and who it's for.
Target audience: [who uses it].
Aesthetic: [visual vibe — e.g. "minimal and focused", "warm and cozy", "bold and energetic"].

### Asset reference
App screenshots live in `core_pics/` — use them as visual reference when generating images.

---

## Image rules
- Size: ALWAYS 1024×1536 (portrait). Never landscape.
- Every prompt starts with: `iPhone photo.`
- Include: `Portrait orientation. Realistic lighting, natural phone camera quality.`
- Style: [describe the visual mood — lighting, color palette, atmosphere]
- No people. No faces.

---

## Text overlay rules
- Font: bold, 6.5% of image height
- Position: lower third, above bottom 12% (avoids TikTok UI)
- Slide 1: white text with black shadow
- Slides 2–6: use brand color per slide (see palette below)

### Brand color palette
| Slide | Color     | Hex       | Mood              |
|-------|-----------|-----------|-------------------|
| 2     | [Name]    | `#000000` | [what it conveys] |
| 3     | [Name]    | `#000000` | [what it conveys] |
| 4     | [Name]    | `#000000` | [what it conveys] |
| 5     | [Name]    | `#000000` | [what it conveys] |
| 6     | [Name]    | `#000000` | [what it conveys] |

---

## Hook formula
Lead with identity or conflict — never with features.

**Works:** `[Person] + [doubt] + [result]`
> "My friend said I'd never stick to it — so I showed her my streak"

**Avoid:** feature-forward copy
> "This app has streaks and session tracking" ← no one cares

---

## Caption formula
```
[Hook restated as a personal story, 2–3 sentences. Journal entry tone, not ad copy.]

[One soft app mention — benefit first, no hard sell.]

[Open question to drive comments — about their goals, not the app.]

#hashtag1 #hashtag2 #hashtag3 #hashtag4 #hashtag5
```

---

## Session ID convention
`{activity}-{mood}-{MMDD}` — e.g. `coding-morning-0317`

---

## Failure log
Keep a running list of what didn't work and why — Claude reads this to avoid repeating mistakes.
- Landscape images → black bars on TikTok. Always portrait.
- Feature-focused hooks → low engagement. Lead with feeling/identity.
