// https://nuxt.com/docs/api/configuration/nuxt-config
import tailwindcss from "@tailwindcss/vite";

export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  app: {
    head: {
      title: 'Stashly',
      link: [
        { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' },
        { rel: 'apple-touch-icon', href: '/apple-touch-icon.svg' },
      ],
    },
  },
  css: ['./app/assets/css/main.css'],
  modules: ['@bg-dev/nuxt-naiveui', '@nuxtjs/i18n'],
  runtimeConfig: {
    public: {
      // Overridable with NUXT_PUBLIC_API_BASE
      apiBase: 'http://127.0.0.1:3000',
      // Overridable with NUXT_PUBLIC_COOKIE_SECURE. Set true only behind HTTPS.
      cookieSecure: false,
    },
  },
  naiveui: {
    colorModePreference: 'light',
    themeConfig: {
      shared: {
        common: {
          primaryColor: '#14b8a6',
          primaryColorHover: '#0d9488',
          primaryColorPressed: '#0f766e',
          primaryColorSuppl: '#14b8a6',
          fontFamily: 'Saira, sans-serif',
        },
      },
    },
  },
  i18n: {
    locales: [
      { code: 'uz', name: "O'zbekcha", file: 'uz.json' },
      { code: 'ru', name: 'Русский', file: 'ru.json' },
      { code: 'en', name: 'English', file: 'en.json' },
    ],
    defaultLocale: 'en',
    langDir: '../i18n/locales',
    strategy: 'no_prefix',
    detectBrowserLanguage: {
      useCookie: true,
      cookieKey: 'i18n_locale',
      fallbackLocale: 'en',
      alwaysRedirect: false,
    },
  },
  vite: {
    plugins: [
      tailwindcss(),
    ],
  },
})
