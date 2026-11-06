// The page view-model is embedded as <script type="application/json" id="vm"> (ADR 0034); the page makes no request.
// The producer of the page writes every "<" as \u003c, so JSON.parse of the text gives the original value.
export function loadPayload(doc: Document): unknown {
  const element = doc.getElementById('vm')
  if (!element) throw new Error('#vm is missing')
  return JSON.parse(element.textContent ?? '')
}
