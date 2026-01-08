import type { CapacitorConfig } from '@capacitor/cli';

const config: CapacitorConfig = {
  appId: 'com.tinytracker.app',
  appName: 'tiny tracker',
  webDir: 'dist',
  server: {
    // Enable this for development with live reload
    // url: 'http://localhost:5173',
    // cleartext: true
  },
  plugins: {
    App: {
      // Deep link URL scheme
      urlScheme: 'tinytracker'
    }
  }
};

export default config;
