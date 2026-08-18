import type { StackResult } from '../lib/aggregate'
import { modeLabel, modelLabel } from '../lib/aggregate'
import { formatPercent, formatTokens } from '../lib/format'
import { IconGauge } from './Icons'
import KindChip from './KindChip'
import './DraftGauge.css'

interface DraftGaugeProps {
  /** The selected mode's result: the figure the waterline reads. */
  result: StackResult
  selectedCount: number
}

/*
  Scale geometry, in SVG user units. The column runs from TOP_Y (a full 200k
  window) down to BOT_Y (zero), so the water rises the way a hull sinks.
*/
const VB_W = 220
const VB_H = 400
const COL_X = 56
const COL_W = 64
const TOP_Y = 20
const BOT_Y = 370
const COL_H = BOT_Y - TOP_Y
const MARK_X = COL_X + COL_W
const READOUT_X = MARK_X + 28

/**
 * The scale the gauge is drawn against. Mirrors CONTEXT_WINDOW in
 * lib/aggregate.ts, which is not exported; the fill itself is driven by
 * result.windowFraction, so this constant only labels the ticks.
 */
const WINDOW_TOKENS = 200_000
const TICK_STEP = 10_000
const LABEL_EVERY = 5

/**
 * The draft gauge: the 200k context window drawn as a hull draft scale, with
 * the selected stack's total riding it as a waterline under a Plimsoll mark.
 *
 * Integrity rules this element has to hold, same as the rest of the page:
 * a stack with no measurable total shows no water and says "pending
 * measurement", never a zero waterline; the figure carries its own
 * measured/modeled chip; a stack over the window pins the line at the top and
 * says so rather than silently clipping.
 */
export default function DraftGauge({ result, selectedCount }: DraftGaugeProps) {
  const pending = result.attribution.length === 0
  const state: 'empty' | 'pending' | 'ready' = selectedCount === 0 ? 'empty' : pending ? 'pending' : 'ready'

  const fraction = result.windowFraction / 100
  const overloaded = state === 'ready' && fraction > 1
  const filled = Math.max(0, Math.min(fraction, 1))
  // The water rect and the rider are drawn at the top of the column and
  // pushed down: one transitionable transform instead of animated geometry.
  const offset = (1 - filled) * COL_H

  const ticks = []
  for (let i = 0; i * TICK_STEP <= WINDOW_TOKENS; i += 1) {
    const y = BOT_Y - (i / (WINDOW_TOKENS / TICK_STEP)) * COL_H
    const major = i % LABEL_EVERY === 0
    ticks.push(
      <line key={`t${i}`} className={major ? 'gauge__tick gauge__tick--major' : 'gauge__tick'} x1={major ? COL_X - 16 : COL_X - 10} y1={y} x2={COL_X} y2={y} />,
    )
    if (major) {
      const tokens = i * TICK_STEP
      ticks.push(
        <text key={`l${i}`} className="gauge__tick-label" x={COL_X - 22} y={y + 4} textAnchor="end">
          {tokens === 0 ? '0' : `${tokens / 1000}k`}
        </text>,
      )
    }
  }

  let ariaLabel: string
  if (state === 'empty') {
    ariaLabel = 'Draft gauge. No servers selected, so there is nothing to weigh.'
  } else if (state === 'pending') {
    ariaLabel = `Draft gauge. ${modeLabel(result.mode)} is pending measurement, so there is no total to report.`
  } else {
    ariaLabel = `Draft gauge. ${formatTokens(result.totalTokens)} tokens, ${formatPercent(result.windowFraction)} of a 200k context window.${overloaded ? ' Overloaded: the stack does not fit the window.' : ''}`
  }

  return (
    <section className="gauge plate" aria-labelledby="gauge-heading">
      <header className="gauge__head">
        <h3 className="stencil gauge__heading" id="gauge-heading">
          <IconGauge size={14} />
          Draft gauge
        </h3>
        <p className="gauge__context">
          <span className="gauge__context-mode">{modeLabel(result.mode)}</span>
          <span className="gauge__context-sep">/</span>
          <span>{modelLabel(result.model)}</span>
          {state === 'ready' && <KindChip kind={result.kind} />}
        </p>
      </header>

      <svg
        className={`gauge__svg${overloaded ? ' gauge__svg--overloaded' : ''}`}
        viewBox={`0 0 ${VB_W} ${VB_H}`}
        preserveAspectRatio="xMidYMid meet"
        role="img"
        aria-label={ariaLabel}
      >
        <defs>
          <clipPath id="gauge-column-clip">
            <rect x={COL_X} y={TOP_Y} width={COL_W} height={COL_H} />
          </clipPath>
        </defs>

        <rect className="gauge__column" x={COL_X} y={TOP_Y} width={COL_W} height={COL_H} />
        {ticks}

        {state === 'ready' && (
          <>
            <g clipPath="url(#gauge-column-clip)">
              <rect
                className="gauge__water"
                x={COL_X}
                y={TOP_Y}
                width={COL_W}
                height={COL_H}
                style={{ transform: `translateY(${offset}px)` }}
              />
            </g>

            <g className="gauge__rider" style={{ transform: `translateY(${offset}px)` }}>
              <line className="gauge__surface" x1={COL_X} y1={TOP_Y} x2={COL_X + COL_W} y2={TOP_Y} />
              {/* The Plimsoll mark itself, riding the waterline. */}
              <circle className="gauge__mark" cx={MARK_X} cy={TOP_Y} r="9" />
              <line className="gauge__mark" x1={MARK_X - 19} y1={TOP_Y} x2={MARK_X + 19} y2={TOP_Y} />
              <text className="gauge__readout-tokens" x={READOUT_X} y={TOP_Y - 5}>
                {formatTokens(result.totalTokens)}
              </text>
              <text className="gauge__readout-pct" x={READOUT_X} y={TOP_Y + 12}>
                {formatPercent(result.windowFraction)}
              </text>
            </g>
          </>
        )}

        {/* Stamped top left, clear of the pinned readout on the right. */}
        {overloaded && (
          <text className="gauge__overloaded" x={4} y={12}>
            OVERLOADED
          </text>
        )}

        {state !== 'ready' && (
          <text className="gauge__quiet" x={VB_W / 2} y={BOT_Y - 12} textAnchor="middle">
            {state === 'empty' ? 'no servers selected' : 'pending measurement'}
          </text>
        )}
      </svg>

      {overloaded && (
        <p className="gauge__note gauge__note--warn">
          This stack does not fit the window. It is over by {formatTokens(result.totalTokens - WINDOW_TOKENS)} tokens
          before a single message is sent.
        </p>
      )}
      {state === 'pending' && (
        <p className="gauge__note">
          No selected server has a {modeLabel(result.mode).toLowerCase()} figure for this model. A missing measurement
          is not a zero, so the gauge stays dry.
        </p>
      )}
    </section>
  )
}
