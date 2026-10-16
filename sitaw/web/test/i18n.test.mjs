// Both dictionaries have the same keys and the same {placeholders}; Polish
// plurals cover the forms Intl.PluralRules picks.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import en from '../static/js/lang/en.js';
import pl from '../static/js/lang/pl.js';

const forms = (v) => (typeof v === 'object' ? Object.values(v) : [v]);
const vars = (v) => new Set(forms(v).flatMap((s) => [...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1])));

test('same keys in en and pl', () => {
  assert.deepEqual(Object.keys(pl).sort(), Object.keys(en).sort());
});

test('same placeholders', () => {
  for (const k of Object.keys(en)) assert.deepEqual(vars(pl[k]), vars(en[k]), k);
});

test('plural maps have every form the language uses', () => {
  for (const [lang, dict] of [['en', en], ['pl', pl]]) {
    const need = new Intl.PluralRules(lang).resolvedOptions().pluralCategories;
    for (const [k, v] of Object.entries(dict)) {
      if (typeof v !== 'object') continue;
      for (const c of need) assert.ok(c in v, `${lang} ${k}: missing ${c}`);
    }
  }
});

test('every key used in the app exists', async () => {
  const { readFile } = await import('node:fs/promises');
  const src = (await Promise.all(['app.js', 'layers.js'].map((f) => readFile(new URL(`../static/js/${f}`, import.meta.url), 'utf8')))).join('\n')
    + await readFile(new URL('../static/index.html', import.meta.url), 'utf8');
  const used = new Set([
    ...[...src.matchAll(/\bt\(\s*'([\w.]+)'/g)].map((m) => m[1]),
    ...[...src.matchAll(/data-i18n(?:-\w+)?="([\w.]+)"/g)].map((m) => m[1]),
  ]);
  for (const k of used) assert.ok(k in en, `missing key ${k}`);
});
