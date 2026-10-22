import love from 'eslint-config-love'
import react from 'eslint-plugin-react'

export default [
  { ignores: ['build/', 'node_modules/'] },
  {
    ...love,
    files: ['src/**/*.{ts,tsx}'],
  },
  {
    ...react.configs.flat.recommended,
    files: ['src/**/*.{ts,tsx}'],
    settings: { react: { version: 'detect' } },
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    rules: {
      quotes: ['error', 'single'],
      'no-multi-spaces': 'error',
      '@typescript-eslint/explicit-function-return-type': 'off',
      '@typescript-eslint/no-floating-promises': 'off',
      '@typescript-eslint/strict-boolean-expressions': 'off',
      '@typescript-eslint/prefer-nullish-coalescing': 'off',
      '@typescript-eslint/no-confusing-void-expression': 'off',
      '@typescript-eslint/no-unused-vars': 'warn',
      '@typescript-eslint/no-non-null-assertion': 'off',
      '@typescript-eslint/consistent-type-assertions': 'off',
      'react/no-unknown-property': 'off',
      // Added by eslint-config-love after standard-with-typescript 34; kept
      // off so the upgrade does not change what the old config enforced.
      eqeqeq: ['error', 'always', { null: 'ignore' }],
      '@typescript-eslint/no-deprecated': 'warn',
      '@typescript-eslint/no-magic-numbers': 'off',
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-unsafe-member-access': 'off',
      '@typescript-eslint/no-unsafe-assignment': 'off',
      '@typescript-eslint/no-unsafe-type-assertion': 'off',
      '@typescript-eslint/no-unsafe-call': 'off',
      '@typescript-eslint/no-unsafe-return': 'off',
      '@typescript-eslint/no-unsafe-argument': 'off',
      '@typescript-eslint/no-import-type-side-effects': 'off',
      '@typescript-eslint/no-unnecessary-condition': 'off',
      '@typescript-eslint/no-unnecessary-type-arguments': 'off',
      '@typescript-eslint/no-unnecessary-type-assertion': 'off',
      '@typescript-eslint/prefer-destructuring': 'off',
      '@typescript-eslint/non-nullable-type-assertion-style': 'off',
    },
  },
]
