// ESLint flat config (ADR 0034, Enforcement). Only ESLint core and typescript-eslint rules are used
// (`no-restricted-imports`, `no-restricted-syntax`, `no-restricted-globals`). Every message starts with a tag.
// The dependency list is checked by scripts/check-frontend-deps.mjs against frontend-whitelist.json.
import tseslint from 'typescript-eslint'

const restrictedImports = {
  patterns: [
    {
      group: ['echarts', 'echarts/*'],
      message: '[second-library] Charts go through recharts and the shadcn chart component (ADR 0034).',
    },
    {
      group: ['framer-motion', 'react-icons/*', '@mui/*', '@chakra-ui/*', '@mantine/*', '@emotion/*'],
      message: '[second-library] A second library for a job already covered (ADR 0034).',
    },
  ],
}

// Base components live in src/components/ui; everything else imports them from there, never radix-ui.
const outsideUiImports = {
  patterns: [
    ...restrictedImports.patterns,
    {
      group: ['radix-ui', '@radix-ui/*'],
      message: '[base-component] Use the component in src/components/ui, not radix-ui.',
    },
  ],
}

const syntaxEverywhere = [
  {
    selector: "JSXAttribute[name.name='dangerouslySetInnerHTML']",
    message: '[inner-html] dangerouslySetInnerHTML is forbidden.',
  },
  {
    selector:
      'MemberExpression[property.name=/^(fetch|XMLHttpRequest|WebSocket|EventSource|sendBeacon)$/]',
    message: '[network] The browser makes no network request; the payload is embedded in the page.',
  },
  {
    selector: 'ImportSpecifier[imported.name=/^(getGroupedRowModel|getExpandedRowModel)$/]',
    message: '[table-aggregation] No TanStack grouping or aggregation; Python sends every figure.',
  },
  {
    selector:
      "CallExpression[callee.property.name='reduce']:has(BinaryExpression[operator='+'], AssignmentExpression[operator='+='])",
    message: '[reduce-sum] No sum over displayed values; Python sends every figure.',
  },
]

const globals = ['fetch', 'XMLHttpRequest', 'WebSocket', 'EventSource'].map((name) => ({
  name,
  message: '[network] The browser makes no network request; the payload is embedded in the page.',
}))

export default tseslint.config(
  { ignores: ['dist/**', 'node_modules/**', 'src/generated/**'] },
  ...tseslint.configs.recommended,
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/components/ui/**'],
    rules: {
      'no-restricted-imports': ['error', outsideUiImports],
      'no-restricted-syntax': ['error', ...syntaxEverywhere],
      'no-restricted-globals': ['error', ...globals],
    },
  },
  {
    files: ['src/components/ui/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': ['error', restrictedImports],
      'no-restricted-syntax': ['error', ...syntaxEverywhere],
      'no-restricted-globals': ['error', ...globals],
    },
  },
)
