/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_APP_ENV?: 'dev' | 'prod' | string;
  readonly VITE_APP_NAMESPACE?: string;
  readonly VITE_APP_TITLE?: string;
  readonly VITE_APP_VERSION?: string;
  readonly VITE_GLOB_API_URL?: string;
  readonly VITE_SERVER_HOST?: string;
  readonly VITE_SERVER_PORT?: string;
  readonly VITE_SERVER_PROXY_TARGET?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
