import { mount } from 'svelte'
import './app.css'
import { setupI18n } from './lib/i18n';
import { waitLocale } from 'svelte-i18n';
import { initDiagnostics } from './lib/diagnostics/bootstrap';
import App from './App.svelte'

// Initialize the breadcrumb pipeline before anything else so console
// patching captures app-startup logs and fetch wrapping covers early
// network traffic (i18n locale load, auth restore, etc.).
await initDiagnostics();

setupI18n();
await waitLocale();

const app = mount(App, {
  target: document.getElementById('app')!,
})

export default app
