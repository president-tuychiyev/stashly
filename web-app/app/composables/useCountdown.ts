/**
 * Simple second-by-second countdown, used for the "resend code" cooldown.
 * The interval is cleared when the owning scope is disposed.
 */
export function useCountdown(defaultSeconds = OTP_RESEND_SECONDS) {
  // Deadline-based so the remaining time is correct even after the tab was
  // backgrounded (browsers throttle `setInterval` while hidden).
  const deadline = ref(0)
  const remaining = ref(0)
  let timer: ReturnType<typeof setInterval> | undefined

  const tick = () => {
    remaining.value = Math.max(0, Math.ceil((deadline.value - Date.now()) / 1000))
    if (remaining.value === 0) stop()
  }

  function stop() {
    clearInterval(timer)
    timer = undefined
    deadline.value = 0
    remaining.value = 0
  }

  const start = (seconds = defaultSeconds) => {
    stop()
    deadline.value = Date.now() + seconds * 1000
    remaining.value = seconds
    if (import.meta.server) return
    timer = setInterval(tick, 1000)
  }

  const active = computed(() => remaining.value > 0)

  onScopeDispose(stop)

  return { remaining, active, start, stop }
}
