export function useDarkMode() {
  const { colorMode, colorModePreference } = useNaiveColorMode()

  const isDark = computed(() => colorMode.value === 'dark')

  const toggleDark = () => {
    colorModePreference.set(isDark.value ? 'light' : 'dark')
  }

  return { isDark, toggleDark }
}
