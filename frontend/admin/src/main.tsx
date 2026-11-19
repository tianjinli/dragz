import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './i18n';
import './styles/base.css';
import './styles/tables.css';
import './styles/layout.css';
import './styles/drawer.css';
import './styles/controls.css';
import './styles/navigation.css';
import './styles/forms.css';
import './styles/auth.css';
import { initLocale } from './stores/locale';
import { useSystemConfigStore } from './stores/system-config';
import { initAutohideScrollbar } from './utils/scrollbar-autohide';

initAutohideScrollbar();

async function bootstrap() {
  // Resolve the persisted language before the first render, so the shell, the
  // antd locale and the Accept-Language of the first requests all agree.
  await initLocale();
  // Best-effort: datetime display falls back to the default pattern until this resolves.
  void useSystemConfigStore.getState().refresh();
  ReactDOM.createRoot(document.getElementById('root')!).render(<React.StrictMode><App/></React.StrictMode>);
}

void bootstrap();
