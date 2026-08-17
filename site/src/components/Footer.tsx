import type { RunInfo } from '../types'
import './Footer.css'

interface FooterProps {
  run: RunInfo
  unreachableCount: number
}

export default function Footer({ run, unreachableCount }: FooterProps) {
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
        <nav className="site-footer__links">
          <a href="./docs/methodology">Methodology</a>
          <a href="./docs/governance">Governance</a>
          <a href="./docs/corrections">Corrections log</a>
        </nav>
        <p className="site-footer__failures">
          Failures publish as data. {unreachableCount} server{unreachableCount === 1 ? '' : 's'} unreachable this run.
        </p>
      </div>
    </footer>
  )
}
