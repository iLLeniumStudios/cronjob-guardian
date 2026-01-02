import { themes as prismThemes } from "prism-react-renderer";
import type { Config } from "@docusaurus/types";
import type * as Preset from "@docusaurus/preset-classic";

const config: Config = {
  title: "CronJob Guardian",
  tagline: "Never miss a failed CronJob again",
  favicon: "img/favicon.ico",

  future: {
    v4: true,
  },

  url: "https://illeniumstudios.github.io",
  baseUrl: "/cronjob-guardian/",

  organizationName: "iLLeniumStudios",
  projectName: "cronjob-guardian",
  deploymentBranch: "gh-pages",
  trailingSlash: false,

  onBrokenLinks: "throw",

  markdown: {
    format: "md",
    hooks: {
      onBrokenMarkdownLinks: "warn",
    },
  },

  i18n: {
    defaultLocale: "en",
    locales: ["en"],
  },

  presets: [
    [
      "classic",
      {
        docs: {
          sidebarPath: "./sidebars.ts",
          editUrl:
            "https://github.com/iLLeniumStudios/cronjob-guardian/tree/main/docs/",
        },
        blog: false,
        theme: {
          customCss: "./src/css/custom.css",
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: "img/social-card.png",
    colorMode: {
      defaultMode: "dark",
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: "CronJob Guardian",
      logo: {
        alt: "CronJob Guardian Logo",
        src: "img/logo.svg",
      },
      items: [
        {
          type: "docSidebar",
          sidebarId: "docsSidebar",
          position: "left",
          label: "Docs",
        },
        {
          href: "https://github.com/iLLeniumStudios/cronjob-guardian",
          label: "GitHub",
          position: "right",
        },
      ],
    },
    footer: {
      style: "dark",
      links: [
        {
          title: "Documentation",
          items: [
            {
              label: "Getting Started",
              to: "/docs/getting-started/introduction",
            },
            {
              label: "Features",
              to: "/docs/features/dead-man-switch",
            },
            {
              label: "Configuration",
              to: "/docs/configuration/monitors/selectors",
            },
          ],
        },
        {
          title: "Reference",
          items: [
            {
              label: "CRD Reference",
              to: "/docs/reference/crds/api-reference",
            },
            {
              label: "Helm Values",
              to: "/docs/reference/helm-values",
            },
            {
              label: "REST API",
              to: "/docs/reference/rest-api",
            },
          ],
        },
        {
          title: "More",
          items: [
            {
              label: "GitHub",
              href: "https://github.com/iLLeniumStudios/cronjob-guardian",
            },
            {
              label: "Releases",
              href: "https://github.com/iLLeniumStudios/cronjob-guardian/releases",
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} CronJob Guardian. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ["bash", "yaml", "json", "go"],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
