const previewCSP = "default-src 'none'; style-src 'unsafe-inline'; img-src data:; font-src data:; script-src 'none'; base-uri 'none'; form-action 'none'"

export function getSVGPreviewDocument(svg: string) {
  return '<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="'
    + previewCSP
    + '"><style>html,body{height:100%;margin:0}body{display:grid;place-items:center;overflow:auto;background:#fff}body>svg{max-width:100%;max-height:100%}</style></head><body>'
    + svg + '</body></html>'
}
