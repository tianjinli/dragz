#!/usr/bin/env node
/**
 * Frontend i18n gate.
 *
 * 1. No Chinese literals in production source (comments, tests and
 *    src/locales/** are exempt; blocks wrapped in i18n-exempt-start/end are
 *    exempt too, e.g. needles matched against backend data). Labels,
 *    placeholders and titles written as English literals are caught too —
 *    the rule is "no hardcoded copy in either language".
 * 2. zh.json and en.json expose exactly the same key set.
 * 3. Every key referenced through t('...'), i18n.t('...') or
 *    <Trans i18nKey="..."> exists in both locale files.
 *
 * Run: pnpm check:i18n  (part of the quality gate)
 */
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('..', import.meta.url));
const srcDir = join(root, 'src');
const localesDir = join(srcDir, 'locales');
const CJK = /[\u3400-\u9fff]/;
// English copy hiding in a UI attribute (`aria-label="Close"` and friends).
// Format masks such as `YYYY-MM-DD HH:mm:ss` contain no run of four lowercase
// letters, so they are not treated as copy.
const EN_COPY_ATTR = /\b(?:aria-label|placeholder|title)="([A-Za-z][^"]*)"/g;
const WORD = /[a-z]{4,}/;
const problems = [];

function walk(dir, out = []) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      if (full === localesDir) continue;
      walk(full, out);
    } else if (/\.(ts|tsx)$/.test(entry) && !/\.test\.(ts|tsx)$/.test(entry)) {
      out.push(full);
    }
  }
  return out;
}

function stripExemptBlocks(source) {
  return source.replace(/\/\* i18n-exempt-start[\s\S]*?i18n-exempt-end \*\//g, (block) =>
    block.replace(/[^\n]/g, ' '),
  );
}

function stripComments(source) {
  return source
    .replace(/\/\*[\s\S]*?\*\//g, (block) => block.replace(/[^\n]/g, ' '))
    .replace(/(^|[^:'"`\\])\/\/.*$/gm, '$1');
}

function lineOf(source, index) {
  return source.slice(0, index).split('\n').length;
}

const files = walk(srcDir);
for (const file of files) {
  const raw = readFileSync(file, 'utf8');
  const code = stripComments(stripExemptBlocks(raw));
  const lines = code.split('\n');
  lines.forEach((line, index) => {
    if (CJK.test(line)) {
      problems.push(`${relative(root, file).split(sep).join('/')}:${index + 1} hardcoded Chinese: ${line.trim().slice(0, 100)}`);
    }
    const label = relative(root, file).split(sep).join('/') + `:${index + 1}`;
    for (const match of line.matchAll(EN_COPY_ATTR)) {
      if (WORD.test(match[1])) problems.push(`${label} hardcoded English copy: ${match[0].slice(0, 100)}`);
    }
  });
}

const usedKeys = new Set();
for (const file of files) {
  const raw = readFileSync(file, 'utf8');
  const patterns = [
    /\bt\(\s*['"]([^'"`$\\]+)['"](?!\s*\+)/g,
    /i18n\.t\(\s*['"]([^'"`$\\]+)['"](?!\s*\+)/g,
    /\bt\(\s*`([^`$\\]+)`(?!\s*\+)/g,
    /i18nKey=\{?\s*['"]([^'"`$\\]+)['"](?!\s*\+)/g,
  ];
  for (const pattern of patterns) {
    for (const match of raw.matchAll(pattern)) {
      if (match[1].includes('${') || match[1].endsWith('.')) continue;
      usedKeys.add(match[1]);
    }
  }
}

function flatten(object, prefix = '', out = new Map()) {
  for (const [key, value] of Object.entries(object)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === 'object' && !Array.isArray(value)) flatten(value, path, out);
    else out.set(path, value);
  }
  return out;
}

const zh = flatten(JSON.parse(readFileSync(join(localesDir, 'zh.json'), 'utf8')));
const en = flatten(JSON.parse(readFileSync(join(localesDir, 'en.json'), 'utf8')));

for (const key of zh.keys()) if (!en.has(key)) problems.push(`en.json is missing key "${key}"`);
for (const key of en.keys()) if (!zh.has(key)) problems.push(`zh.json is missing key "${key}"`);
for (const key of usedKeys) {
  if (!zh.has(key)) problems.push(`t('${key}') is not defined in zh.json`);
  if (!en.has(key)) problems.push(`t('${key}') is not defined in en.json`);
}

if (problems.length > 0) {
  for (const problem of problems) console.error(`check-i18n: ${problem}`);
  console.error(`check-i18n: ${problems.length} problem(s)`);
  process.exit(1);
}

console.log(`check-i18n: ok (${usedKeys.size} keys used, ${zh.size} keys per locale)`);
