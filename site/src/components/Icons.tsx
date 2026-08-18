/*
  Hand-rolled 16px icon set: 1.5px strokes, currentColor, no icon library and
  no network request. Every icon here is used somewhere on the page; the set
  is deliberately small.
*/

import type { ReactNode } from 'react'

interface IconProps {
  /** Rendered size in px. Icons are drawn on a 16-unit grid. */
  size?: number
  className?: string
}

interface FrameProps extends IconProps {
  children: ReactNode
}

function Frame({ size = 16, className, children }: FrameProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.5}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
      className={className}
    >
      {children}
    </svg>
  )
}

export function IconGauge(props: IconProps) {
  return (
    <Frame {...props}>
      <path d="M2.6 12.2a5.6 5.6 0 1 1 10.8 0" />
      <path d="M8 12.2 11 8.6" />
      <path d="M2.6 12.2h1.6M11.8 12.2h1.6" />
    </Frame>
  )
}

export function IconLink(props: IconProps) {
  return (
    <Frame {...props}>
      <path d="M6.6 9.4a2.9 2.9 0 0 0 4.2 0l2-2a2.9 2.9 0 0 0-4.1-4.1l-1 1" />
      <path d="M9.4 6.6a2.9 2.9 0 0 0-4.2 0l-2 2a2.9 2.9 0 0 0 4.1 4.1l1-1" />
    </Frame>
  )
}

export function IconExternal(props: IconProps) {
  return (
    <Frame {...props}>
      <path d="M7 3.4H3.4v9.2h9.2V9" />
      <path d="M9.6 2.9h3.5v3.5" />
      <path d="m13.1 2.9-5.3 5.3" />
    </Frame>
  )
}

export function IconWarning(props: IconProps) {
  return (
    <Frame {...props}>
      <path d="M8 2.6 14.4 13.4H1.6z" />
      <path d="M8 6.6v3" />
      <path d="M8 11.7h.01" />
    </Frame>
  )
}

export function IconCheck(props: IconProps) {
  return (
    <Frame {...props}>
      <path d="m3.2 8.4 3.2 3.2 6.4-7.2" />
    </Frame>
  )
}

export function IconX(props: IconProps) {
  return (
    <Frame {...props}>
      <path d="m4.2 4.2 7.6 7.6M11.8 4.2l-7.6 7.6" />
    </Frame>
  )
}

/** Modeled: an estimate produced in the lab, not read off the surface. */
export function IconFlask(props: IconProps) {
  return (
    <Frame {...props}>
      <path d="M6.4 2v4.3l-3 5.3a1.3 1.3 0 0 0 1.1 2h7a1.3 1.3 0 0 0 1.1-2l-3-5.3V2" />
      <path d="M5.4 2h5.2" />
      <path d="M4.7 9.9h6.6" />
    </Frame>
  )
}

/** Measured: taken off the real surface with a rule. */
export function IconRuler(props: IconProps) {
  return (
    <Frame {...props}>
      <rect x="1.4" y="5.2" width="13.2" height="5.6" rx="0.8" />
      <path d="M4.4 5.2v2.1M7 5.2v3M9.6 5.2v2.1M12.2 5.2v3" />
    </Frame>
  )
}

/** Filesystem fallback: no brand icon exists for the reference filesystem
 *  server, so this reads as a folder on the same 16-unit grid. */
export function IconFolder(props: IconProps) {
  return (
    <Frame {...props}>
      <path d="M2 4.6a1 1 0 0 1 1-1h2.8l1.3 1.6H13a1 1 0 0 1 1 1v5.6a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1z" />
    </Frame>
  )
}

/**
 * The Plimsoll mark: a circle with a horizontal line through its centre
 * running past both sides. The brand mark, and the same shape that rides the
 * waterline on the draft gauge.
 */
export function PlimsollMark({ size = 20, className }: IconProps) {
  return (
    <svg
      width={size}
      height={size * 0.75}
      viewBox="0 0 32 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      aria-hidden="true"
      focusable="false"
      className={className}
    >
      <circle cx="16" cy="12" r="7" />
      <path d="M2 12h28" strokeLinecap="round" />
    </svg>
  )
}
