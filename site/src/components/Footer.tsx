import type { RunInfo, ServerStatus } from '../types'
import { IconExternal } from './Icons'
import './Footer.css'

interface FooterProps {
  run: RunInfo
  /**
   * Count per non-ok status, keyed by failure class (methodology section 7).
   * All six classes count as failures, not just `unreachable`.
   */
  failuresByClass: Partial<Record<ServerStatus, number>>
  serverCount: number
}

export default function Footer({ run, failuresByClass, serverCount }: FooterProps) {
  const entries = Object.entries(failuresByClass) as [ServerStatus, number][]
  const failureCount = entries.reduce((sum, [, n]) => sum + n, 0)
  const breakdown = entries.map(([cls, n]) => `${cls}: ${n}`).join(', ')

  return (
    <footer className="site-footer">
      <div className="rule-strip" aria-hidden="true" />
      <div className="container site-footer__inner">
        <p className="site-footer__failures" title={breakdown ? `By class: ${breakdown}` : undefined}>
          Failures publish as data. <span className="num">{failureCount}</span> of{' '}
          <span className="num">{serverCount}</span> servers failed this run.
        </p>

        <dl className="site-footer__meta">
          <div>
            <dt className="stencil">Methodology</dt>
            <dd className="num">v{run.methodology_version}</dd>
          </div>
          <div>
            <dt className="stencil">Run date</dt>
            <dd className="num">{run.date}</dd>
          </div>
          <div>
            <dt className="stencil">Harness</dt>
            <dd className="num">v{run.harness_version}</dd>
          </div>
        </dl>

        {/*
          Absolute GitHub URLs, not site-relative paths: the site is a single
          page with no router, so ./docs/... would 404. These resolve once the
          repo goes public at launch.
        */}
        <nav className="site-footer__links">
          <a href="https://github.com/lopster568/loadline/blob/main/docs/methodology-v0.md">
            Methodology
            <IconExternal size={13} />
          </a>
          {/*
            Tier 1 and Tier 2 are specified in separate documents and the page
            now shows figures from both, so both are linked. The anchor lands
            on the mode-label rule, which is what a reader chasing a measured
            or modeled chip is after.
          */}
          <a href="https://github.com/lopster568/loadline/blob/main/docs/methodology-v0.md#3-client-mode-modeling">
            Mode labels
            <IconExternal size={13} />
          </a>
          <a href="https://github.com/lopster568/loadline/blob/main/docs/tier2-task-suites.md">
            Tier 2 suite
            <IconExternal size={13} />
          </a>
          <a href="https://github.com/lopster568/loadline/tree/main/governance">
            Governance
            <IconExternal size={13} />
          </a>
          <a href="https://github.com/lopster568/loadline/blob/main/governance/corrections-log.md">
            Corrections log
            <IconExternal size={13} />
          </a>
        </nav>
      </div>
    </footer>
  )
}
