import type { Config } from 'tailwindcss';

const config: Config = {
  content: ['./app/**/*.{ts,tsx}', './components/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        background: {
          DEFAULT: 'rgb(var(--color-background))',
          surface: 'rgb(var(--color-surface))',
          'surface-elevated': 'rgb(var(--color-surface-elevated))',
        },
        border: 'rgb(var(--color-border))',
        text: {
          primary: 'rgb(var(--color-text-primary))',
          secondary: 'rgb(var(--color-text-secondary))',
          muted: 'rgb(var(--color-text-muted))',
        },
        success: 'rgb(var(--color-success))',
        warning: 'rgb(var(--color-warning))',
        danger: 'rgb(var(--color-danger))',
        info: 'rgb(var(--color-info))',
      },
    },
    darkMode: 'class',
  },
  plugins: [],
};

export default config;
