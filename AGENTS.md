# TikTok Slideshow Skill

## What I do
Generate 6-slide TikTok photo carousels promoting a productivity / dream-achieving app using the `generate_slideshow` MCP tool.
Each slideshow: 6 portrait images (1024x1536), hook text on slide 1, value text on slides 2-6, posted to TikTok drafts via Postiz.

---

## App context
**FocusHero** — a gamified focus timer built on deliberate practice theory. Every session counts toward 10,000-hour mastery of a skill, with 13 hero levels from Aspirant to Transcendent.
Target audience: students, developers, musicians, designers, and self-improvers who want to track their path to real expertise.
Aesthetic: purposeful and epic — hero journey energy, not hustle culture. Think "aesthetic deep-work vlog meets RPG progression" — focused, cinematic, achievable.
Skill tags in the app: Coding, Reading, Design, Spanish, Piano, Prototyping, Market Research, Value.

### App screenshots
Real screenshots are available in `core_pics/` — use these as visual reference for the app's UI, screens, and aesthetic when generating slideshows.
- `Main (started).png` — home/timer screen during an active session (light mode)
- `Main (started) dark.png` — home/timer screen during an active session (dark mode)
- `Todos.png` — todo list view
- `Todos (Expanded).png` — expanded todo detail
- `Habit Tracker.png` — habit tracking screen
- `Analytics.png` — analytics / time tracking view (light mode)
- `Analytics (dark).png` — analytics / time tracking view (dark mode)
- `Analytics - building (dark).png` — analytics view mid-session, stats accumulating in real time (dark mode)
- `Hero Levels.png` — hero level reference (older 2-column grid layout, sage green accent): Aspirant 0h → Novice Focus 5h → Apprentice 25h → Skilled Fighter 75h → Elite Warrior 150h → Champion 300h → Legend 500h → Master 750h → Grandmaster 1,250h → Sage 2,000h
- `Hero journey Card.png` — Hero Journeys per-skill detail card (light mode, violet accent): skill tabs at top (e.g. Value, UI/UX), current level badge (e.g. Level 3 "Skilled Fighter"), Next Level Progress bar with hours (e.g. 100h/160h, 66%), stat tiles for Total Hours / Sessions / Time Today / Today's Sessions, Copy Stats button, Level Reference preview at bottom
- `Hero journey Card (dark).png` — same per-skill detail card in dark mode with warm amber accent (e.g. Marketing tab active, Level 3, 74% progress, 112h/150h, 278 sessions)
- `Hero journey Levels.png` — Level Reference, redesigned zigzag connected-path layout (light mode, violet accent): Aspirant 0h → Novice Focus → Apprentice → Skilled Fighter (current, dashed border) → Elite Warrior → Champion → Legend → Master → Grandmaster, with arrows showing progression
- `Hero journey Levels (dark).png` — same zigzag level path in dark mode with amber accent
- `Time Since.png` — Time Since tracker: elapsed time since each tracked event (meals, stretches, reps, supplements), with filter tabs and optional countdowns to future dates
- `Countdown Widget.png` — iOS countdown widget
- `Lock Screen Widget.png` — iOS lock screen widget
- `AppStore.png` — App Store listing screenshot
- `icon.png` — app icon

---

## Image rules
- Size: ALWAYS 1024x1536 (portrait). Never landscape. Never square.
- Model: gemini-3.1-flash-image-preview (fast) or gemini-3-pro-image-preview (quality)
- Every prompt must start with: `iPhone photo.`
- Include: `Portrait orientation. Realistic lighting, natural phone camera quality.`
- Style: purposeful deep-work atmosphere with a subtle epic quality — like a hero mid-journey. Soft ambient light, intentional minimal setup. Colour palette drawn from FocusHero's brand: sky blue, sage green, soft violet, warm amber, muted coral. Mastery feels earned, not gifted. Focused, not hustle-culture.
- No people. No faces. Hands only if essential for context (e.g., hands on piano keys, fingers on keyboard).
- Images can draw from FocusHero skill tag activities (Coding, Reading, Design, Spanish, Piano, Prototyping, Market Research, Value) but don't have to — any scene that fits the deep-work / hero-journey aesthetic works.
- Colour-grade each image to echo the slide's brand colour where possible (e.g., cool blue tones for Coding, green for Reading/growth, violet for Design/vision).

---

## /last30days integration

When the user says **"trending"** or **"using /last30days"** (for slideshows, brand pics, or captions), run the `/last30days` skill first before generating anything.

### What to research
Run `/last30days` on a query like:
- `productivity app TikTok hooks viral copy 2026`
- `TikTok marketing trends productivity 2026`
- `[specific angle the user mentioned] TikTok hooks 2026`

### How to apply the research
After research completes, extract the top hook frameworks and map them across the 6 slides (or 6 brand pics). Each image should use a **different** hook type — don't repeat the same formula.

**Hook type → slide mapping (default):**
| Slide | Hook type | Example from last session |
|-------|-----------|--------------------------|
| 1 (hook) | Identity call-out | "If your notes app is your to-do list, this is for you." |
| 2 | Results-first | "I built 8 habits in 30 days. Here's what actually stuck." |
| 3 | Curiosity gap | "The scheduling mistake quietly killing your productivity." |
| 4 | Pattern interrupt | "Your to-do list isn't the problem. Your system is." |
| 5 | FOMO / reveal | "I found where 3 hours a day were disappearing." |
| 6 (CTA) | Gamification | "What if getting things done felt like leveling up?" |

Override this mapping with whatever the research actually surfaces. If a new hook pattern appears in the data, use it and note it here.

### What to look for in research output
- **Trending hook formats** — specific sentence structures beating benchmarks right now
- **Trending themes** — e.g. "Admin Night", "365 buttons", cozy productivity aesthetics
- **Vocabulary to use** — words and phrases with high engagement (e.g. "streak", "level up", "disappearing")
- **Vocabulary to avoid** — clichés flagged as low-engagement (e.g. "hustle", "grind", "crush it")
- **Platform-specific signals** — what's working on TikTok vs Reels right now

### Research → caption
Apply the same research to the caption. If research surfaces a trending narrative format (e.g. journal-entry confessional, "I tested X for 30 days"), use it to rewrite the caption formula for that batch.

---

## Hook formulas (ranked by performance)
1. **[Person] + [doubt/conflict] + [result they saw]** → best. 100K+ views consistently.
   - "My friend said I'd never actually finish my goals — so I showed her my 90-day streak"
   - "My partner didn't believe I was being productive until I showed him this"
   - "I told my mum I was working on myself and she laughed — then I showed her this"

2. **Before/after identity shift** → good. 30-70K views.
   - "This is what 30 days of actually tracking your time does to you"
   - "I used to think I was lazy. Turns out I just wasn't measuring anything."

3. **Feature-forward** → avoid. Under 5K views every time.
   - "This app tracks your sessions" ← flopped
   - "Productivity app with streaks" ← flopped

**Rule: always ask "who is the other person and what is the conflict?"**

---

## Text overlay rules
- Font: bold, 6.5% of image height (~100px)
- Position: lower third, above bottom 12% (TikTok UI zone)
- Max line width: 82% of image width (auto word-wraps)
- Every slide gets overlay text
- Slide 1: hook text — white with black shadow (always)
- Slides 2-6: productivity value text — use app brand colors below

### App color palette (for slides 2-6)
| Slide | Color name    | Hex       | Tone                  |
|-------|---------------|-----------|-----------------------|
| 2     | Sky Blue      | `#85BDEB` | Focus / Deep work     |
| 3     | Sage Green    | `#8CC799` | Growth / Consistency  |
| 4     | Soft Violet   | `#AD85D9` | Vision / Goals        |
| 5     | Warm Amber    | `#EBC285` | Reflection / Insight  |
| 6     | Muted Coral   | `#EB9E85` | CTA / Momentum        |

Additional palette (use as needed):
- Soft Lavender `#B89EE6` — Mindset / Values
- Soft Teal `#73C7BD` — Learning / Skills
- Golden Yellow `#E6D680` — Achievement / Practice

### Productivity value text (per slide)
Short, punchy, benefit-led — not feature-led. Speaks to the dream, not the tool.
- Slide 2: "Track every session. See exactly where your time goes."
- Slide 3: "Build streaks across everything you're working toward."
- Slide 4: "Color-coded by goal. Your whole week, at a glance."
- Slide 5: "Patterns show when you're in flow — and when you're not."
- Slide 6: "Start your first session in 10 seconds. Free."

---

## Caption formula
```
[Hook restated as personal story, 2-3 sentences. Feel like a journal entry, not an ad.]

[One soft mention of app — benefit-first, no hard sell.]

[Open question to drive comments — about their goals, not the app.]

#productivity #studywithme #goalsetting #selfimprovement #deepwork
```
Max 5 hashtags. TikTok current limit.

---

## Session ID convention
`{activity}-{mood}-{MMDD}` — e.g. `coding-morning-0317`, `reading-evening-0318`

---

## Failure log
- Landscape images (1536x1024) → black bars on every TikTok. Always use portrait.
- People or faces → inconsistent across slides, uncanny. Avoid entirely.
- Font below 6% height → unreadable on phone. Never go below 6%.
- Hook text too high → hidden by TikTok status bar. Keep in lower third.
- Feature-focused hooks → dead. Nobody cares about features. Lead with identity/feeling.
- Motivational clichés ("hustle", "grind") → low engagement. Use specific, honest language.
