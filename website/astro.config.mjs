import { defineConfig, fontProviders } from "astro/config";
import starlight from "@astrojs/starlight";
import sitemap from "@astrojs/sitemap";

import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  site: "https://go-workflow-auditlog.lars.software",

  compressHTML: true,

  prefetch: {
    prefetchAll: false,
    defaultStrategy: "hover",
  },

  fonts: [
    {
      provider: fontProviders.google(),
      name: "Space Grotesk",
      cssVariable: "--font-space-grotesk",
      weights: [300, 400, 500, 600, 700],
      styles: ["normal"],
      subsets: ["latin"],
      fallbacks: ["sans-serif"],
    },
    {
      provider: fontProviders.fontsource(),
      name: "JetBrains Mono",
      cssVariable: "--font-jetbrains-mono",
      weights: [400, 500, 600, 700],
      styles: ["normal"],
      subsets: ["latin"],
      fallbacks: ["monospace"],
    },
  ],

  integrations: [
    sitemap(),
    starlight({
      title: "go-workflow-auditlog",
      favicon: "/favicon.svg",
      customCss: ["./src/styles/starlight.css"],
      expressiveCode: {
        themes: ["github-light", "github-dark"],
        frames: {
          showCopyToClipboardButton: true,
        },
      },
      sidebar: [
        {
          label: "Getting Started",
          items: [
            { label: "Installation", slug: "getting-started/installation" },
            { label: "Quick Start", slug: "getting-started/quick-start" },
          ],
        },
        {
          label: "Guides",
          items: [
            { label: "Event Stream", slug: "guides/event-stream" },
            { label: "Export Formats", slug: "guides/export-formats" },
            { label: "Filtering & Diffing", slug: "guides/filtering-and-diffing" },
            { label: "HTML Dashboard", slug: "guides/html-dashboard" },
          ],
        },
        {
          label: "API Reference",
          items: [
            { label: "Public API", slug: "api-reference" },
            {
              label: "Full API on pkg.go.dev",
              link: "https://pkg.go.dev/github.com/larsartmann/go-workflow-auditlog",
            },
          ],
        },
        {
          label: "Community",
          items: [
            { label: "Changelog", slug: "changelog" },
            { label: "Contributing", slug: "contributing" },
            { label: "Related Tools", slug: "related-tools" },
          ],
        },
      ],
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/larsartmann/go-workflow-auditlog",
        },
      ],
      head: [
        {
          tag: "meta",
          attrs: {
            name: "description",
            content:
              "Audit logging for Azure/go-workflow. Records every step execution event with timestamped events, export to JSON, NDJSON, diagrams, and an interactive HTML dashboard.",
          },
        },
      ],
    }),
  ],

  vite: {
    plugins: [tailwindcss()],
  },
});
