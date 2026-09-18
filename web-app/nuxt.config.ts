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
    // `dir: 'rtl'` is what app.vue stamps on <html>; everything else is LTR.
    locales: [
      { code: 'uz', language: 'uz-UZ', name: "O'zbekcha", file: 'uz.json' },
      { code: 'ru', language: 'ru-RU', name: 'Русский', file: 'ru.json' },
      { code: 'en', language: 'en-US', name: 'English', file: 'en.json' },
      { code: 'kk', language: 'kk-KZ', name: 'Қазақша', file: 'kk.json' },
      { code: 'ky', language: 'ky-KG', name: 'Кыргызча', file: 'ky.json' },
      { code: 'tg', language: 'tg-TJ', name: 'Тоҷикӣ', file: 'tg.json' },
      { code: 'az', language: 'az-AZ', name: 'Azərbaycanca', file: 'az.json' },
      { code: 'tr', language: 'tr-TR', name: 'Türkçe', file: 'tr.json' },
      { code: 'uk', language: 'uk-UA', name: 'Українська', file: 'uk.json' },
      { code: 'pl', language: 'pl-PL', name: 'Polski', file: 'pl.json' },
      { code: 'de', language: 'de-DE', name: 'Deutsch', file: 'de.json' },
      { code: 'fr', language: 'fr-FR', name: 'Français', file: 'fr.json' },
      { code: 'es', language: 'es-ES', name: 'Español', file: 'es.json' },
      { code: 'pt', language: 'pt-PT', name: 'Português', file: 'pt.json' },
      { code: 'it', language: 'it-IT', name: 'Italiano', file: 'it.json' },
      { code: 'zh', language: 'zh-CN', name: '中文', file: 'zh.json' },
      { code: 'ja', language: 'ja-JP', name: '日本語', file: 'ja.json' },
      { code: 'ko', language: 'ko-KR', name: '한국어', file: 'ko.json' },
      { code: 'hi', language: 'hi-IN', name: 'हिन्दी', file: 'hi.json' },
      { code: 'id', language: 'id-ID', name: 'Bahasa Indonesia', file: 'id.json' },
      { code: 'vi', language: 'vi-VN', name: 'Tiếng Việt', file: 'vi.json' },
      { code: 'ar', language: 'ar-SA', name: 'العربية', file: 'ar.json', dir: 'rtl' },
      { code: 'fa', language: 'fa-IR', name: 'فارسی', file: 'fa.json', dir: 'rtl' },
      { code: 'he', language: 'he-IL', name: 'עברית', file: 'he.json', dir: 'rtl' },
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
