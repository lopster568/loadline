/*
  NOTICE: brand marks below are sourced from Simple Icons (CC0 1.0, no
  attribution required). Trademarks belong to their respective owners; they
  are used here only to identify the measured products, never as an
  endorsement claim. https://simpleicons.org

  Only the icons actually referenced on the page are imported by name, so an
  unused mark never enters the bundle.

  Not every measured server or model has a Simple Icons entry (as of the
  version pinned in package.json, Playwright and OpenAI do not). Those, plus
  any id this map has never seen, fall through to a generic mark so a server
  never renders with an empty slot where its icon should be.
*/

import type { CSSProperties } from 'react'
import {
  siGithub,
  siNotion,
  siLinear,
  siStripe,
  siSentry,
  siCloudflare,
  siKubernetes,
  siPostgresql,
  siJaeger,
  siGooglechrome,
  siClaude,
  siGooglegemini,
} from 'simple-icons'
import type { SimpleIcon } from 'simple-icons'
import { IconFolder, IconLink } from './Icons'
import './BrandIcon.css'

interface BrandIconProps {
  /** A server id (types.ts ServerEntry.id) or a model id (types.ts ModelId). */
  id: string
  /** Rendered size in px. 16 on server cards, 14 on the model switch and the manifest name column. */
  size?: number
  className?: string
}

// Corpus id -> Simple Icons mark. Keys are the exact ids the run's data.json
// and lib/aggregate's ModelId union use, not the brand's own slug.
const BRAND_ICONS: Record<string, SimpleIcon> = {
  github: siGithub,
  notion: siNotion,
  linear: siLinear,
  stripe: siStripe,
  sentry: siSentry,
  cloudflare: siCloudflare,
  kubernetes: siKubernetes,
  postgres: siPostgresql,
  jaeger: siJaeger,
  'chrome-devtools': siGooglechrome,
  claude: siClaude,
  gemini: siGooglegemini,
}

/*
  Brand colour resolution. Every mark renders in its own hex by default; the
  only override is a legibility guard, computed once per icon from the hex
  itself (pure math, no deps) rather than tuned by eye per brand:

    - dark theme reads the mark off --plate, which is near-black. A hex
      whose relative luminance sits under DARK_SAFE_FLOOR (github #181717,
      notion #000000, sentry's near-black purple, and the handful of mid
      blues/purples that land just under the line too) would nearly
      disappear there, so it's lightened toward white instead of falling
      back to grey - the hue stays recognisable, just lifted.
    - light theme reads the mark off paper. A hex whose luminance sits over
      LIGHT_SAFE_CEIL would wash out there, so it's darkened the same way.
      Nothing in the current corpus is that light, but the guard exists for
      whatever brand shows up next.

  Both substitutes are 75% mixes toward white/black respectively: enough to
  clear the floor/ceiling with margin, not so much the mark reads as tinted
  grey.
*/
const DARK_SAFE_LUMINANCE_FLOOR = 0.18
const LIGHT_SAFE_LUMINANCE_CEIL = 0.85
const SAFE_MIX_AMOUNT = 0.75

function hexToRgb(hex: string): [number, number, number] {
  const n = parseInt(hex, 16)
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
}

// WCAG relative luminance (the same formula behind contrast-ratio checks),
// 0 (black) to 1 (white).
function relativeLuminance(hex: string): number {
  const [r, g, b] = hexToRgb(hex)
  const toLinear = (c: number) => {
    const cs = c / 255
    return cs <= 0.03928 ? cs / 12.92 : Math.pow((cs + 0.055) / 1.055, 2.4)
  }
  return 0.2126 * toLinear(r) + 0.7152 * toLinear(g) + 0.0722 * toLinear(b)
}

// Mix a hex colour toward white (target 255) or black (target 0) by amount
// (0..1), channel by channel. Returns a bare hex string, no leading '#'.
function mixToward(hex: string, target: number, amount: number): string {
  const [r, g, b] = hexToRgb(hex)
  const mixChannel = (c: number) => Math.round(c + (target - c) * amount).toString(16).padStart(2, '0')
  return `${mixChannel(r)}${mixChannel(g)}${mixChannel(b)}`
}

interface ResolvedBrandColor {
  /** The brand's own hex, unmodified - used in both themes when it's already safe. */
  brand: string
  /** What to render in dark theme: the brand hex, or a lightened stand-in if it's too dark to read on --plate. */
  darkSafe: string
  /** What to render in light theme: the brand hex, or a darkened stand-in if it's too light to read on paper. */
  lightSafe: string
}

// Resolved once at module load, not per render: each entry is pure hex math
// over BRAND_ICONS, so there's nothing here that benefits from recomputing.
const BRAND_COLORS: Record<string, ResolvedBrandColor> = Object.fromEntries(
  Object.entries(BRAND_ICONS).map(([id, icon]) => {
    const hex = icon.hex
    const luminance = relativeLuminance(hex)
    const darkSafe = luminance < DARK_SAFE_LUMINANCE_FLOOR ? mixToward(hex, 255, SAFE_MIX_AMOUNT) : hex
    const lightSafe = luminance > LIGHT_SAFE_LUMINANCE_CEIL ? mixToward(hex, 0, SAFE_MIX_AMOUNT) : hex
    const resolved: ResolvedBrandColor = { brand: `#${hex}`, darkSafe: `#${darkSafe}`, lightSafe: `#${lightSafe}` }
    return [id, resolved]
  }),
)

// CSSProperties doesn't know about custom properties; BrandIcon hands the
// three brand colour tokens to CSS through inline style rather than JS-side
// theme detection, so the .brand-icon__mark rules in BrandIcon.css can pick
// the right one per theme the same way every other token in the app does.
type BrandIconStyle = CSSProperties & {
  '--brand': string
  '--brand-dark-safe': string
  '--brand-light-safe': string
}

function firstLetter(id: string): string {
  const match = id.match(/[a-zA-Z]/)
  return match ? match[0].toUpperCase() : '?'
}

export default function BrandIcon({ id, size = 16, className }: BrandIconProps) {
  const brand = BRAND_ICONS[id]
  if (brand) {
    const colors = BRAND_COLORS[id]
    const style: BrandIconStyle = {
      '--brand': colors.brand,
      '--brand-dark-safe': colors.darkSafe,
      '--brand-light-safe': colors.lightSafe,
    }
    return (
      <svg
        width={size}
        height={size}
        viewBox="0 0 24 24"
        aria-hidden="true"
        focusable="false"
        className={`brand-icon__mark${className ? ` ${className}` : ''}`}
        style={style}
      >
        <path d={brand.path} />
      </svg>
    )
  }

  // Filesystem and fetch have obvious generic marks in the existing hand-
  // rolled set; use those instead of a bare letter.
  if (id === 'filesystem') return <IconFolder size={size} className={className} />
  if (id === 'fetch') return <IconLink size={size} className={className} />

  // Universal fallback: context7, playwright, openai_o200k (no Simple Icons
  // entry as of this package version), and any future unknown id. Stencil
  // square with the id's first letter, never an empty slot.
  return (
    <span
      className={`brand-icon__letter${className ? ` ${className}` : ''}`}
      style={{ width: size, height: size, fontSize: Math.round(size * 0.62) }}
      aria-hidden="true"
    >
      {firstLetter(id)}
    </span>
  )
}
