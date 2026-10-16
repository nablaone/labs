// UI language: English (default) and Polish.
//
// t('key', { name: 'X' }) looks the key up in the current language, falling
// back to English, and fills {placeholders}. A value may be a plural map
// ({ one, few, many, other }) chosen by vars.n with Intl.PluralRules.
// Values are trusted markup; callers escape what they pass in vars.
// Numbers keep a dot as the decimal separator in every language.

import en from './lang/en.js';
import pl from './lang/pl.js';

export const LANGS = { en: 'EN', pl: 'PL' };
const DICTS = { en, pl };
const LANG_KEY = 'sitaw.lang';

let lang = 'en';
try { const l = localStorage.getItem(LANG_KEY); if (DICTS[l]) lang = l; } catch { /* private mode */ }
document.documentElement.lang = lang;

export const getLang = () => lang;

export function setLang(l) {
  if (!DICTS[l]) return;
  lang = l;
  document.documentElement.lang = l;
  try { localStorage.setItem(LANG_KEY, l); } catch { /* private mode */ }
  translateDom(document);
}

export const has = (key) => key in en;

export function t(key, vars = {}) {
  let s = DICTS[lang][key] ?? en[key] ?? key;
  if (typeof s === 'object') s = s[new Intl.PluralRules(lang).select(vars.n ?? 0)] ?? s.other;
  return s.replace(/\{(\w+)\}/g, (m, k) => (k in vars ? String(vars[k]) : m));
}

// Static markup carries its keys: data-i18n (text), data-i18n-aria
// (aria-label), data-i18n-title (title), data-i18n-ph (placeholder).
export function translateDom(root) {
  for (const el of root.querySelectorAll('[data-i18n]')) el.textContent = t(el.dataset.i18n);
  for (const el of root.querySelectorAll('[data-i18n-aria]')) el.setAttribute('aria-label', t(el.dataset.i18nAria));
  for (const el of root.querySelectorAll('[data-i18n-title]')) el.title = t(el.dataset.i18nTitle);
  for (const el of root.querySelectorAll('[data-i18n-ph]')) el.placeholder = t(el.dataset.i18nPh);
}

// A server error in the current language: its code when we know it, else the
// server's English message.
export function errText(ex) {
  return ex?.code && has(`err.${ex.code}`) ? t(`err.${ex.code}`) : ex?.message ?? String(ex);
}
