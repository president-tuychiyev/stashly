<script setup lang="ts">
defineOptions({ inheritAttrs: false })
defineProps<{ markOnly?: boolean }>()
// The logo can be rendered twice on one page (sidebar + drawer); SVG ids must stay unique
// or the browser resolves the gradient/mask from a hidden copy and draws nothing.
const uid = useId()
const gradientId = `stashly-cg-${uid}`
const maskId = `stashly-gap-${uid}`
</script>

<template>
  <span class="inline-flex items-center gap-2" :class="$attrs.class">
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" class="h-full w-auto shrink-0" aria-hidden="true">
      <g>
        <defs><linearGradient :id="gradientId" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#14b8a6"/><stop offset="1" stop-color="#0f766e"/></linearGradient>
        <mask :id="maskId"><rect width="512" height="512" fill="#fff"/><rect x="150" y="380" width="212" height="90" fill="#000"/></mask></defs>
        <path d="M400 240c-8.89-89.54-71-144-144-144c-69 0-113.44 48.2-128 96c-60 6-112 43.59-112 112c0 66 54 112 120 112h260c55 0 100-27.44 100-88c0-59.82-53-85.76-96-88z" fill="none" :stroke="`url(#${gradientId})`" stroke-width="40" stroke-linejoin="round" :mask="`url(#${maskId})`"/>
        <g transform="translate(256,265)">
          <path d="M-96-60 0-100 96-60 0-20Z" fill="#2dd4bf"/>
          <path d="M-96-60 0-20v40l-96-40Z" fill="#0f766e"/><path d="M96-60 0-20v40l96-40Z" fill="#14b8a6"/>
          <path d="M-96-4 0 36v40l-96-40Z" fill="#0f766e"/><path d="M96-4 0 36v40l96-40Z" fill="#14b8a6"/>
          <path d="M-96 52 0 92v40l-96-40Z" fill="#0f766e"/><path d="M96 52 0 92v40l96-40Z" fill="#14b8a6"/>
        </g>
      </g>
    </svg>
    <span v-if="!markOnly" class="text-2xl font-extrabold tracking-tight text-teal-700 dark:text-teal-300">stashly</span>
    <span v-else class="sr-only">Stashly</span>
  </span>
</template>
