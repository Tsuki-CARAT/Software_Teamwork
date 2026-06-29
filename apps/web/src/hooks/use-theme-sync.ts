import { useEffect } from 'react'

import { useThemeStore } from '@/stores/theme-store'

/* -------------------------------------------------------------------------- */
/*  Types                                                                     */
/* -------------------------------------------------------------------------- */

export type ColorKey =
  'neutral' | 'blue' | 'green' | 'purple' | 'orange' | 'red' | 'teal' | 'pink' | 'yellow' | 'slate'

export type ColorPreset = {
  light: string
  dark: string
  label: string
}

/* -------------------------------------------------------------------------- */
/*  Constants                                                                 */
/* -------------------------------------------------------------------------- */

export const PRIMARY_COLORS: Record<ColorKey, ColorPreset> = {
  neutral: {
    light: 'oklch(0.205 0 0)',
    dark: 'oklch(0.922 0 0)',
    label: '默认',
  },
  blue: {
    light: 'oklch(0.546 0.245 255)',
    dark: 'oklch(0.623 0.214 259.815)',
    label: '蓝色',
  },
  green: {
    light: 'oklch(0.527 0.154 145)',
    dark: 'oklch(0.627 0.194 149.214)',
    label: '绿色',
  },
  purple: {
    light: 'oklch(0.511 0.262 302)',
    dark: 'oklch(0.627 0.265 303.9)',
    label: '紫色',
  },
  orange: {
    light: 'oklch(0.554 0.195 42)',
    dark: 'oklch(0.646 0.222 41.116)',
    label: '橙色',
  },
  red: {
    light: 'oklch(0.577 0.245 27.325)',
    dark: 'oklch(0.704 0.191 22.216)',
    label: '红色',
  },
  teal: {
    light: 'oklch(0.6 0.118 184.704)',
    dark: 'oklch(0.696 0.17 162.48)',
    label: '青色',
  },
  pink: {
    light: 'oklch(0.592 0.249 0.584)',
    dark: 'oklch(0.704 0.191 355.513)',
    label: '粉色',
  },
  yellow: {
    light: 'oklch(0.681 0.162 75.834)',
    dark: 'oklch(0.795 0.184 86.047)',
    label: '黄色',
  },
  slate: {
    light: 'oklch(0.446 0.03 256.8)',
    dark: 'oklch(0.708 0.01 256.8)',
    label: '灰蓝',
  },
}

export const FONT_SIZE_SCALES: Record<'small' | 'medium' | 'large', number> = {
  small: 0.875,
  medium: 1,
  large: 1.125,
}

/* -------------------------------------------------------------------------- */
/*  Hook                                                                      */
/* -------------------------------------------------------------------------- */

export function useThemeSync() {
  const mode = useThemeStore((s) => s.mode)
  const primaryColor = useThemeStore((s) => s.primaryColor)
  const radius = useThemeStore((s) => s.radius)
  const fontSize = useThemeStore((s) => s.fontSize)

  useEffect(() => {
    const root = document.documentElement

    /* Dark mode class */
    if (mode === 'dark') {
      root.classList.add('dark')
    } else {
      root.classList.remove('dark')
    }

    /* Injected style element for CSS variables that vary between :root and .dark */
    const preset = PRIMARY_COLORS[primaryColor as ColorKey]
    if (preset) {
      const styleId = 'theme-custom-override'
      let style = document.getElementById(styleId) as HTMLStyleElement | null
      if (!style) {
        style = document.createElement('style')
        style.id = styleId
        document.head.appendChild(style)
      }
      style.textContent = [
        ':root {',
        `  --primary: ${preset.light};`,
        '  --primary-foreground: oklch(0.985 0 0);',
        `  --radius: ${radius}rem;`,
        `  --font-size-scale: ${FONT_SIZE_SCALES[fontSize]};`,
        '}',
        '.dark {',
        `  --primary: ${preset.dark};`,
        '  --primary-foreground: oklch(0.205 0 0);',
        '}',
      ].join('\n')
    }
  }, [mode, primaryColor, radius, fontSize])
}
