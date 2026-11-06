// Checks the dependencies of the package.json files against frontend-whitelist.json (the copy of the
// frontend-stack whitelist) and web/deps-allowlist.txt. node built-ins only: no install, no network.
//
// A dependency fails when its name is not a key of the whitelist, when its version is not equal to the
// pin of that key (so `^`, `~` and `npm:` aliases fail), when its name starts with `@radix-ui/` (use the
// `radix-ui` package), or when web/deps-allowlist.txt does not list its name.
//
// Usage: node scripts/check-frontend-deps.mjs [repository-root]
import { existsSync, readFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(process.argv[2] ?? join(fileURLToPath(import.meta.url), '..', '..'))
const read = (file) => readFileSync(join(root, file), 'utf8')

const whitelist = JSON.parse(read('frontend-whitelist.json')).packages
const allowed = new Set(
  read('web/deps-allowlist.txt')
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line !== '' && !line.startsWith('#')),
)

const problems = []
for (const file of ['package.json', 'web/package.json']) {
  if (!existsSync(join(root, file))) continue
  const pkg = JSON.parse(read(file))
  for (const section of ['dependencies', 'devDependencies']) {
    for (const [name, version] of Object.entries(pkg[section] ?? {})) {
      const where = `${file} ${section} ${name}@${version}`
      if (name.startsWith('@radix-ui/')) {
        problems.push(`${where}: use the radix-ui package, not @radix-ui/*`)
      }
      if (!Object.hasOwn(whitelist, name)) {
        problems.push(`${where}: not in frontend-whitelist.json`)
      } else if (whitelist[name].version !== version) {
        problems.push(`${where}: the whitelist pins ${whitelist[name].version}`)
      }
      if (!allowed.has(name)) {
        problems.push(`${where}: not in web/deps-allowlist.txt`)
      }
    }
  }
}

if (problems.length > 0) {
  for (const problem of problems) console.error(`check-frontend-deps: ${problem}`)
  process.exit(1)
}
console.log('check-frontend-deps: ok')
