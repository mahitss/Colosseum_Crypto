import type { Config } from 'tailwindcss';

const config: Config = {
  content: ['./app/**/*.{ts,tsx}', './components/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        background: {
          DEFAULT: 'rgb(var(--color-background))',
          elevated: 'rgb(var(--color-background-elevated))',
        },
        surface: {
          DEFAULT: 'rgb(var(--color-surface))',
          elevated: 'rgb(var(--color-surface-elevated))',
          hover: 'rgb(var(--color-surface-hover))',
        },
        border: {
          DEFAULT: 'rgb(var(--color-border))',
          strong: 'rgb(var(--color-border-strong))',
          subtle: 'rgb(var(--color-border-subtle))',
        },
        text: {
          primary: 'rgb(var(--color-text-primary))',
          secondary: 'rgb(var(--color-text-secondary))',
          muted: 'rgb(var(--color-text-muted))',
          inverse: 'rgb(var(--color-text-inverse))',
        },
        accent: {
          DEFAULT: 'rgb(var(--color-accent))',
          hover: 'rgb(var(--color-accent-hover))',
          muted: 'rgb(var(--color-accent-muted))',
        },
        success: 'rgb(var(--color-success))',
        warning: 'rgb(var(--color-warning))',
        danger: 'rgb(var(--color-danger))',
        info: 'rgb(var(--color-info))',
        severity: {
          critical: 'rgb(var(--color-severity-critical))',
          significant: 'rgb(var(--color-severity-significant))',
          watch: 'rgb(var(--color-severity-watch))',
          info: 'rgb(var(--color-severity-info))',
        },
      },
      fontFamily: {
        sans: ['var(--font-sans)'],
        mono: ['var(--font-mono)'],
        display: ['var(--font-display)'],
      },
      fontSize: {
        'xs': ['0.75rem', { lineHeight: '1.5' }],
        'sm': ['0.875rem', { lineHeight: '1.5' }],
        'base': ['1rem', { lineHeight: '1.5' }],
        'lg': ['1.125rem', { lineHeight: '1.5' }],
        'xl': ['1.25rem', { lineHeight: '1.5' }],
        '2xl': ['1.5rem', { lineHeight: '1.4' }],
        '3xl': ['1.875rem', { lineHeight: '1.3' }],
        '4xl': ['2.25rem', { lineHeight: '1.2' }],
      },
      spacing: {
        '1': '0.25rem',
        '2': '0.5rem',
        '3': '0.75rem',
        '4': '1rem',
        '5': '1.25rem',
        '6': '1.5rem',
        '8': '2rem',
        '10': '2.5rem',
        '12': '3rem',
        '16': '4rem',
      },
      borderRadius: {
        'sm': '0.25rem',
        'md': '0.375rem',
        'lg': '0.5rem',
        'xl': '0.75rem',
        '2xl': '1rem',
      },
      boxShadow: {
        'sm': '0 1px 2px 0 rgb(0 0 0 / 0.05)',
        'md': '0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)',
        'lg': '0 10px 15px -3px rgb(0 0 0 / 0.1), 0 4px 6px -4px rgb(0 0 0 / 0.1)',
        'xl': '0 20px 25px -5px rgb(0 0 0 / 0.1), 0 8px 10px -6px rgb(0 0 0 / 0.1)',
      },
      transitionDuration: {
        'fast': '100ms',
        'normal': '200ms',
        'slow': '300ms',
      },
      transitionTimingFunction: {
        'ease': 'ease',
      },
      borderWidth: {
        '1': '1px',
        '2': '2px',
      },
      borderColor: {
        DEFAULT: 'rgb(var(--color-border))',
        strong: 'rgb(var(--color-border-strong))',
        subtle: 'rgb(var(--color-border-subtle))',
      },
      backgroundColor: {
        background: 'rgb(var(--color-background))',
        'background-elevated': 'rgb(var(--color-background-elevated))',
        surface: 'rgb(var(--color-surface))',
        'surface-elevated': 'rgb(var(--color-surface-elevated))',
        'surface-hover': 'rgb(var(--color-surface-hover))',
      },
      textColor: {
        primary: 'rgb(var(--color-text-primary))',
        secondary: 'rgb(var(--color-text-secondary))',
        muted: 'rgb(var(--color-text-muted))',
        inverse: 'rgb(var(--color-text-inverse))',
      },
      accentColor: {
        DEFAULT: 'rgb(var(--color-accent))',
        hover: 'rgb(var(--color-accent-hover))',
        muted: 'rgb(var(--color-accent-muted))',
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-conic': 'conic-gradient(from 180deg at 50% 50%, var(--tw-gradient-stops))',
      },
      keyframes: {
        'fade-in': {
          from: { opacity: '0' },
          to: { opacity: '1' },
        },
        'slide-in-from-top': {
          from: { opacity: '0', transform: 'translateY(-4px)' },
          to: { opacity: '1', transform: 'translateY(0)' },
        },
        'slide-in-from-bottom': {
          from: { opacity: '0', transform: 'translateY(4px)' },
          to: { opacity: '1', transform: 'translateY(0)' },
        },
        'pulse-soft': {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0.7' },
        },
      },
      animation: {
        'fade-in': 'fade-in 200ms ease-out',
        'slide-in-from-top': 'slide-in-from-top 200ms ease-out',
        'slide-in-from-bottom': 'slide-in-from-bottom 200ms ease-out',
        'pulse-soft': 'pulse-soft 2s ease-in-out infinite',
      },
    },
    darkMode: 'class',
  },
  plugins: [],
};

export default config;