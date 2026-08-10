/// <reference types="vite/client" />

interface NmConfig {
  apiAuthToken?: string;
}

interface Window {
  NM_CONFIG?: NmConfig;
}
