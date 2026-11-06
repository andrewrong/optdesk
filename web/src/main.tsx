import './index.css'
import { App } from './shell/App'
import { mountShell } from './shell/mount'

mountShell(document, (payload) => <App payload={payload} />)
