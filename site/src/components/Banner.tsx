import { IconWarning } from './Icons'
import './Banner.css'

interface BannerProps {
  visible: boolean
}

/**
 * Persistent, unmissable notice shown whenever the loaded dataset is sample
 * data. This is a hard integrity requirement: sample numbers must never be
 * mistaken for real measurements. Sticky so it stays visible while scrolling
 * through any of the page's three sections.
 */
export default function Banner({ visible }: BannerProps) {
  if (!visible) return null
  return (
    <div className="banner" role="alert">
      <IconWarning size={14} />
      <span className="banner__text">Sample data. These are not measurements.</span>
    </div>
  )
}
