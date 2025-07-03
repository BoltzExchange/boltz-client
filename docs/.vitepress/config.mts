import { defineConfig } from "vitepress";

const docsRoot = "https://docs.boltz.exchange";

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "Boltz Client",
  description: "Boltz Client Docs",
  themeConfig: {
    logo: "./assets/logo.svg",
    search: {
      provider: "local",
    },
    nav: [{ text: "Home", link: docsRoot }],
    sidebar: [
      {
        items: [
          { text: "👋 Introduction", link: "/index" },
          { text: "💰 Wallets", link: "/wallets" },
          { text: "🔁 Autoswap", link: "/autoswap" },
          { text: "🏅 Boltz Pro", link: "/boltz-pro" },
          { text: "🎛️ Configuration", link: "/configuration" },
          { text: "🤖 gRPC API", link: "/grpc" },
          {
            text: "🤖 REST API",
            link: "https://github.com/BoltzExchange/boltz-client/blob/master/pkg/boltzrpc/rest-annotations.yaml",
          },

          { text: "🔙 Home", link: docsRoot },
        ],
      },
    ],
    socialLinks: [
      {
        icon: "github",
        link: "https://github.com/BoltzExchange/boltz-client",
      },
    ],
  },
  // Ignore dead links to localhost
  ignoreDeadLinks: [/https?:\/\/localhost/],
});
