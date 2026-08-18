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

function firstLetter(id: string): string {
  const match = id.match(/[a-zA-Z]/)
  return match ? match[0].toUpperCase() : '?'
}

export default function BrandIcon({ id, size = 16, className }: BrandIconProps) {
  const brand = BRAND_ICONS[id]
  if (brand) {
    return (
      <svg
        width={size}
        height={size}
        viewBox="0 0 24 24"
        fill="currentColor"
        aria-hidden="true"
        focusable="false"
        className={className}
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
