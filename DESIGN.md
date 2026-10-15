# DESIGN.md: Opslira

Direction for Opslira's interface. Transcribed from `docs/referensi-design.md` (primary UI
direction) and `docs/Design.md` (structural token base), both chosen by the product owner.
The agent applies this file as data; it does not invent direction here.

## Identity

Opslira is an internal infrastructure monitor for one office. The people who use it are
sysadmins and IT staff watching seven target categories: network devices, web apps,
domains and TLS, databases, mail servers, object storage, and Beszel-monitored servers.

The interface has one job: **make an unhealthy target impossible to miss, and make the
evidence behind that verdict reachable in one click.** Everything else defers to that.

## Personality

Calm, factual, instrument-panel. It reads like a NOC wall display that a technician
trusts, not a marketing dashboard. Numbers are stated plainly, never dramatized. When
there is no data, the screen says so instead of showing a zero that looks like a fact.

## Theme

Dark, fixed. Reason: this is a monitoring console read for long stretches, often beside
other dark tooling, where a bright field is glare and eye strain. Dark is a working
condition here, not a trend. No light-mode toggle is shipped, so there is no second mode
to break.

PDF reports are the deliberate exception: they print on a light theme because paper is
light. See `docs/referensi-design.md` §6.1.

## Palette

Three cores plus one accent, on a zinc neutral ramp.

| Role | Token | Value | Where it is used |
|---|---|---|---|
| Base | `noc-bg` | `#09090b` | Page background |
| Surface | `noc-surface` | `#131316` | Card and panel fills |
| Border | `noc-border` | `#27272a` | Dividers, card edges, table rules |
| Text | `noc-text` | `#f4f4f5` | Primary text |
| Muted text | `noc-muted` | `#a1a1aa` | Labels, secondary values |
| Accent | `noc-accent` | `#9fe870` | Focus rings, active nav, primary action, the one live indicator |
| Status UP | `status-up` | `#34d399` | Healthy target |
| Status DEGRADED | `status-degraded` | `#fbbf24` | Degraded target |
| Status DOWN | `status-down` | `#fb7185` | Failing target |
| Status MAINTENANCE | `status-maintenance` | `#38bdf8` | Planned maintenance |

Status colors are semantic data, not decoration. They appear only on a real status, and
always next to an icon and a text label so color is never the only carrier of meaning.

The Wise tokens in `tailwind.config.js` (`wise.*`) exist for the light PDF templates only.

### Contrast floor

Every text token must clear WCAG AA (4.5:1 for body text, 3:1 for large text) against the
surface it sits on. The measured floor on `noc-surface`:

- `noc-text` on surface: 16.9:1
- `noc-muted` on surface: 7.2:1
- Accent and all four status colors on surface: above 6.8:1

Nothing below `noc-muted` may carry text. `zinc-600` and darker are decoration only.

## Typography

| Role | Family | Reason |
|---|---|---|
| Display | Geist | Geometric and compact at large weights; the wordmark and page titles read as instrument labels, not editorial headlines |
| Body | Inter | Neutral at small sizes, which is most of this UI |
| Mono | JetBrains Mono | Tabular figures for IPs, ports, latency, percentages, and log timestamps, where digit alignment is a reading aid |

Mono is a data treatment, applied through the `.tabular` utility, never a decorative
heading style. Uppercase labels stay small and are used for field and column labels only,
never as a section eyebrow above a heading.

## Shape and elevation

Radii are a small deliberate set: `xs` 2px for chips, `sm` 10px for inputs and buttons,
`md` 16px for cards and panels. Pill radius is reserved for status badges, where the shape
marks a state token. It is not the default for buttons, inputs, or cards.

Elevation is a ring, not a drop shadow: a 1px border plus a single hairline inset. Nothing
floats. Cards sit on the page; only the drawer and the palette modal lift above it.

## Motion

Motion marks a state change and then stops. Permitted:

- A control's hover and focus transition.
- The drawer and palette entering and leaving.
- The status badge crossfade when a target's status actually changes.
- Tab indicator sliding between tabs.

Not permitted: anything that loops forever with no user action. A permanent pulse or a
travelling border is noise on a screen that is already asking for attention, and it
competes with the one indicator that does mean something.

## Glass

Glass is an accent, used once: the sticky top bar, so content scrolling under it stays
legible. Card surfaces are solid. Blur is never the character of the whole UI.

## Dial

`Dial: ENERGY 2 / RHYTHM 2 / MOTION 1`

- **ENERGY 2 (balanced).** The status verdict is loud; the chrome is quiet.
- **RHYTHM 2 (consistent with a few breaks).** Dashboards share one skeleton so an
  operator's eyes learn it once. The breaks are real: the NOC overview leads with the
  incident list, the network view leads with the traffic question, the mail view leads
  with blacklist state. Each screen's first block is the thing that screen is for.
- **MOTION 1 (hover and transitions only).** Motion is a state change, not an ambience.

## Identity motif

The **status rail**: on every target row and card, status is rendered as one unit, an icon,
a text label, and a tinted rule, always in the same place at the leading edge of the
target's identity. Learn it once on the NOC overview and it reads the same on all seven
target types and in every table. That repetition is what makes the interface recognizably
Opslira rather than a generic admin panel.

## Voice

Plain English for operational copy, and the same plain English for technical field names.
Short sentences. A number appears only when it came from a probe or the database; otherwise
the screen shows `n/a` or an explicit empty state. No claim is written that the data cannot
back.
