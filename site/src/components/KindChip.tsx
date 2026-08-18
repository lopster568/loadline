import type { Kind } from '../types'
import { IconFlask, IconRuler } from './Icons'

/**
 * Provenance chip carried by every number on the page. `measured` came off a
 * real tool surface (ruler); `modeled` is an estimate (flask). Shared by the
 * gauge, the three-mode comparison and the detail view so the two can never
 * label the same figure differently.
 */
export default function KindChip({ kind }: { kind: Kind }) {
  return (
    <span className={`kind-chip kind-chip--${kind}`}>
      {kind === 'measured' ? <IconRuler size={12} /> : <IconFlask size={12} />}
      <span>{kind}</span>
    </span>
  )
}
