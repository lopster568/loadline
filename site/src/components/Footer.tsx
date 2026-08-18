import type { RunInfo, ServerStatus } from '../types'
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
      <div className="container site-footer__inner">
        <div className="site-footer__meta">
          <span>Methodology v{run.methodology_version}</span>
          <span className="site-footer__dot">·</span>
          <span>Run date {run.date}</span>
          <span className="site-footer__dot">·</span>
          <span>Harness v{run.harness_version}</span>
        </div>
        {/*
          Absolute GitHub URLs, not site-relative paths: the site is a single
          page with no router, so ./docs/... would 404. These resolve once the
          repo goes public at launch.
        */}
        <nav className="site-footer__links">
          <a href="https://github.com/lopster568/loadline/blob/main/docs/methodology-v0.md">Methodology</a>
          <a href="https://github.com/lopster568/loadline/tree/main/governance">Governance</a>
          <a href="https://github.com/lopster568/loadline/blob/main/governance/corrections-log.md">Corrections log</a>
        </nav>
        <p className="site-footer__failures" title={breakdown ? `By class: ${breakdown}` : undefined}>
          Failures publish as data. {failureCount} of {serverCount} servers failed this run.
        </p>
      </div>
    </footer>
  )
}
