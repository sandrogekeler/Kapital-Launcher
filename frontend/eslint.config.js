import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'

export default tseslint.config(
  { ignores: ['dist', 'wailsjs', 'coverage'] },
  {
    files: ['scripts/**/*.mjs'],
    extends: [js.configs.recommended],
    languageOptions: { ecmaVersion: 2024, globals: globals.node },
  },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2024,
      globals: globals.browser,
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      'react-hooks/rules-of-hooks': 'error',
      'react-hooks/exhaustive-deps': 'warn',
      'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
      // Convention: an intentionally unused binding starts with `_`.
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_', destructuredArrayIgnorePattern: '^_' },
      ],
      '@typescript-eslint/consistent-type-imports': 'error',
      // Styling goes through Tailwind utilities over the generated token layer.
      // Inline style={{}} is for genuinely computed values only, each with a
      // documented eslint-disable-next-line (.claude/rules/frontend-style.md).
      'no-restricted-syntax': [
        'error',
        {
          selector: "JSXAttribute[name.name='style']",
          message:
            'Prefer Tailwind utility classes over inline style={{}}. Inline styles are only for dynamic/computed values.',
        },
        {
          selector: "ImportDeclaration[source.value='lucide-react']",
          message:
            'Import icons from src/lib/icons.ts, the one module allowed to import lucide-react.',
        },
      ],
    },
  },
  {
    // The one module that owns the icon dependency.
    files: ['src/lib/icons.ts'],
    rules: { 'no-restricted-syntax': 'off' },
  },
)
