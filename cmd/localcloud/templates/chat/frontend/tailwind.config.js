// cmd/localcloud/templates/chat/frontend/tailwind.config.js
/** @type {import('tailwindcss').Config} */
export default {
    content: [
        "./index.html",
        "./src/**/*.{js,ts,jsx,tsx}",
    ],
    darkMode: 'class',
    theme: {
        extend: {
            animation: {
                'bounce': 'bounce 1s infinite',
            },
            typography: {
                DEFAULT: {
                    css: {
                        maxWidth: 'none',
                    },
                },
            },
        },
    },
    plugins: [
        require('@tailwindcss/typography'),
    ],
}