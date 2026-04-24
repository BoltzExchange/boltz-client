import { defineConfig } from "vitepress";

const docsRoot = "https://docs.boltz.exchange";

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "Boltz Client",
  description: "Boltz Client Docs",
  head: [["link", { rel: "icon", href: "/assets/logo.svg" }]],
  themeConfig: {
    logo: "/assets/logo.svg",
    search: {
      provider: "local",
      options: {
        detailedView: true,
      },
    },
    nav: [{ text: "🏠 Docs Home", link: docsRoot, target: "_self" }],
    sidebar: [
      {
        items: [
          { text: "👋 Introduction", link: "/index" },
          { text: "💰 Wallets", link: "/wallets" },
          { text: "🏦 Funding", link: "/funding" },
          { text: "🔁 Autoswap", link: "/autoswap" },
          { text: "🏅 Boltz Pro", link: "/boltz-pro" },
          { text: "🎛️ Configuration", link: "/configuration" },
          { text: "🤖 gRPC API", link: "/grpc" },
          {
            text: "🤖 REST API",
            link: "https://github.com/BoltzExchange/boltz-client/blob/master/pkg/boltzrpc/rest-annotations.yaml",
          },
          { text: "🏠 Docs Home", link: docsRoot, target: "_self" },
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
