import defaultTheme from 'tailwindcss/defaultTheme';

/** @type {import('tailwindcss').Config} */
export default {
    darkMode: 'class',

    content: [
        './resources/**/*.blade.php',
        './resources/**/*.js',
        './resources/**/*.ts',
        './resources/**/*.tsx',
        './app/Modules/**/*.php',
    ],

    theme: {
        extend: {
            colors: {
                // Dark NOC semantic tokens (DESIGN.md, docs/referensi-design.md §3.1)
                noc: {
                    bg: '#09090b',
                    surface: '#131316',
                    raised: '#1b1b1f',
                    border: '#27272a',
                    text: '#f4f4f5',
                    muted: '#a1a1aa',
                    accent: '#9fe870',
                },
                // Status tokens are data, not decoration (DESIGN.md)
                status: {
                    up: '#34d399',
                    degraded: '#fbbf24',
                    down: '#fb7185',
                    maintenance: '#38bdf8',
                },
                // Light-theme tokens for PDF templates only (docs/Design.md)
                wise: {
                    primary: '#9fe870',
                    'on-primary': '#163300',
                    secondary: '#0e0f0c',
                    success: '#054d28',
                    danger: '#d03238',
                    warning: '#ffd11a',
                    info: '#38c8ff',
                    ink: '#0e0f0c',
                    bone: '#f0f1ee',
                    canvas: '#fafafa',
                },
            },

            fontFamily: {
                // DESIGN.md typography: Geist display, Inter body, JetBrains Mono for data
                display: ['Geist', 'Inter', 'sans-serif'],
                sans: ['Inter', 'Helvetica', 'Arial', ...defaultTheme.fontFamily.sans],
                mono: ['JetBrains Mono', 'Geist Mono', ...defaultTheme.fontFamily.mono],
            },

            fontSize: {
                'display-md': ['2.5rem', { lineHeight: '1.05', fontWeight: '700' }],
                'display-sm': ['1.75rem', { lineHeight: '1.1', fontWeight: '700' }],
                'body-lg': ['1.125rem', { lineHeight: '1.5' }],
                'body-md': ['1rem', { lineHeight: '1.5' }],
                'body-sm': ['0.875rem', { lineHeight: '1.55' }],
                'code-md': ['0.875rem', { lineHeight: '1.55' }],
                'code-sm': ['0.75rem', { lineHeight: '1.55' }],
            },

            spacing: {
                xxs: '0.25rem',
                xs: '0.5rem',
                sm: '0.75rem',
                md: '1rem',
                lg: '1.5rem',
                xl: '2rem',
                xxl: '3rem',
                xxxl: '4rem',
                section: '6rem',
            },

            borderRadius: {
                // DESIGN.md shape: xs chips, sm controls, md cards. Pill is status-only.
                xs: '2px',
                sm: '6px',
                md: '10px',
                lg: '14px',
                xl: '20px',
            },

            boxShadow: {
                // Elevation is a ring, not a drop shadow (DESIGN.md)
                ring: '0 0 0 1px rgba(244, 244, 245, 0.08)',
                lift: '0 8px 24px -12px rgba(0, 0, 0, 0.7)',
                focus: '0 0 0 2px #09090b, 0 0 0 4px #9fe870',
            },

            keyframes: {
                'status-in': {
                    from: { opacity: '0' },
                    to: { opacity: '1' },
                },
            },

            animation: {
                'status-in': 'status-in 0.18s ease-out',
            },
        },
    },

    plugins: [],
};
