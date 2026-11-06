import { StrictMode, useEffect, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { ErrorBoundary } from './ErrorBoundary'
import { loadPayload } from '@/vm'

function Committed({ onCommit, children }: { onCommit: () => void; children: ReactNode }) {
  useEffect(onCommit, [onCommit])
  return children
}

// The notice `#fail-closed` is visible in the HTML. It is hidden only after the payload parsed and the
// first React commit; a payload error or a render error leaves or puts it visible again.
export function mountShell(doc: Document, app: (payload: unknown) => ReactNode): Root | null {
  const notice = doc.getElementById('fail-closed')
  const container = doc.getElementById('root')
  if (!notice || !container) return null
  let payload: unknown
  try {
    payload = loadPayload(doc)
  } catch {
    notice.hidden = false
    return null
  }
  const show = () => {
    notice.hidden = false
  }
  const hide = () => {
    notice.hidden = true
  }
  const root = createRoot(container)
  root.render(
    <StrictMode>
      <ErrorBoundary onError={show}>
        <Committed onCommit={hide}>{app(payload)}</Committed>
      </ErrorBoundary>
    </StrictMode>,
  )
  return root
}
