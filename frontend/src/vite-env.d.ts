/// <reference types="vite/client" />

interface GaConfig {
  apiAuthToken?: string;
}

interface Window {
  GA_CONFIG?: GaConfig;
}
