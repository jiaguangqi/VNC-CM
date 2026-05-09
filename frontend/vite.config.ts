import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    host: "0.0.0.0",
    port: 3000,
    hmr: {
      host: "10.10.38.148",
      port: 3000,
    },
    proxy: {
      "/api": {
        target: "http://master-service:8080",
        ws: true,
        changeOrigin: true,
        configure: (proxy) => {
          proxy.on("proxyReq", (proxyReq, req) => {
            const host = req.headers.host;
            if (host) {
              proxyReq.setHeader("X-Forwarded-Host", host);
            }
          });
        },
      },
      "/share": {
        target: "http://master-service:8080",
        ws: true,
        changeOrigin: true,
        configure: (proxy) => {
          proxy.on("proxyReq", (proxyReq, req) => {
            const host = req.headers.host;
            if (host) {
              proxyReq.setHeader("X-Forwarded-Host", host);
            }
          });
        },
      },
    },
  },
});
