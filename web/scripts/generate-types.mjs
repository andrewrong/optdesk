// Regenerates the TypeScript types of web/src/generated from contracts/schemas.
// The set of types is the set of .ts files of src/generated; each argument
// (`page.<name>` or `<name>`, for example `page.macro_rates.v1`) adds a target, so a new schema gets its file.
// Run from the directory web: `node scripts/generate-types.mjs [page.<name> ...]`.
import { execFileSync } from 'node:child_process';
import { existsSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

const GENERATED_DIR = 'src/generated';
const SCHEMAS_DIR = '../contracts/schemas';
const BANNER = '// Code generated from contracts/, DO NOT EDIT.\n/* eslint-disable */';
const NAME = /^[a-z0-9_]+\.v[0-9]+$/;

function fail(message) {
  console.error(`generate-types: ${message}`);
  process.exit(1);
}

function parseArgument(arg) {
  const name = arg.startsWith('page.') ? arg.slice('page.'.length) : arg;
  if (!NAME.test(name)) {
    fail(`argument "${arg}" is not page.<name> or <name> with <name> like market_summary.v2`);
  }
  return name;
}

const targets = new Set();
for (const file of readdirSync(GENERATED_DIR)) {
  if (!file.endsWith('.ts')) continue;
  const name = file.slice(0, -'.ts'.length);
  if (NAME.test(name)) targets.add(name);
}
for (const arg of process.argv.slice(2)) targets.add(parseArgument(arg));

// Check every schema before the first file is written.
for (const name of targets) {
  const schema = join(SCHEMAS_DIR, `page.${name}.json`);
  if (!existsSync(schema)) fail(`schema ${schema} does not exist (target ${name})`);
}

const json2ts = join('node_modules', '.bin', 'json2ts');
for (const name of [...targets].sort()) {
  execFileSync(
    json2ts,
    [
      '-i', join(SCHEMAS_DIR, `page.${name}.json`),
      '-o', join(GENERATED_DIR, `${name}.ts`),
      '--cwd', SCHEMAS_DIR,
      '--bannerComment', BANNER,
    ],
    { stdio: 'inherit' },
  );
}
