import { createI18n } from 'vue-i18n'
import en from './locales/en.json'

export type MessageSchema = typeof en

// Single-locale i18n setup. We keep vue-i18n in place because hundreds of
// components reference $t() and tearing it out is pure churn for no
// user-visible difference.
export const i18n = createI18n({
  legacy: false,
  locale: 'en',
  fallbackLocale: 'en',
  messages: { en },
})
