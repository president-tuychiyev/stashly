<script setup lang="ts">
import { createLocale, darkTheme, enUS } from 'naive-ui'
// Naive UI ships one RTL style sheet per component and only exports them under
// the `unstable` prefix; the config provider mirrors nothing without them.
// Components with no entry here (the menu, for one) have no RTL sheet at all
// and simply follow the document direction.
import {
  unstableAlertRtl,
  unstableAvatarGroupRtl,
  unstableBadgeRtl,
  unstableButtonGroupRtl,
  unstableButtonRtl,
  unstableCardRtl,
  unstableCheckboxRtl,
  unstableCollapseRtl,
  unstableCollapseTransitionRtl,
  unstableDataTableRtl,
  unstableDialogRtl,
  unstableDrawerRtl,
  unstableDynamicInputRtl,
  unstableFlexRtl,
  unstableInputNumberRtl,
  unstableInputOtpRtl,
  unstableInputRtl,
  unstableListRtl,
  unstableMessageRtl,
  unstableNotificationRtl,
  unstablePageHeaderRtl,
  unstablePaginationRtl,
  unstablePopoverRtl,
  unstableRadioRtl,
  unstableRowRtl,
  unstableScrollbarRtl,
  unstableSelectRtl,
  unstableSpaceRtl,
  unstableStatisticRtl,
  unstableStepsRtl,
  unstableTableRtl,
  unstableTagRtl,
  unstableThingRtl,
  unstableTreeRtl,
  unstableTreeSelectRtl,
  unstableUploadsRtl,
} from 'naive-ui'

const { isDark } = useDarkMode()

// Every form field already has a label, so the stock "Please Input" hint is
// noise; fields that need a real hint still pass their own `placeholder`.
const naiveLocale = createLocale({ Input: { placeholder: '' }, InputNumber: { placeholder: '' } }, enUS)

// Arabic, Persian and Hebrew ship with `dir: 'rtl'` in the locale list; the
// attribute has to reach <html> for the whole page to mirror.
const { locale, locales } = useI18n()
const direction = computed(
  () => (locales.value as Array<{ code: string; dir?: string }>).find((item) => item.code === locale.value)?.dir ?? 'ltr',
)

// Handing the provider the sheets only in RTL keeps the LTR render untouched.
const rtlStyles = computed(() => (direction.value === 'rtl'
  ? [
    unstableAlertRtl,
  unstableAvatarGroupRtl,
  unstableBadgeRtl,
  unstableButtonGroupRtl,
  unstableButtonRtl,
  unstableCardRtl,
  unstableCheckboxRtl,
  unstableCollapseRtl,
  unstableCollapseTransitionRtl,
  unstableDataTableRtl,
  unstableDialogRtl,
  unstableDrawerRtl,
  unstableDynamicInputRtl,
  unstableFlexRtl,
  unstableInputNumberRtl,
  unstableInputOtpRtl,
  unstableInputRtl,
  unstableListRtl,
  unstableMessageRtl,
  unstableNotificationRtl,
  unstablePageHeaderRtl,
  unstablePaginationRtl,
  unstablePopoverRtl,
  unstableRadioRtl,
  unstableRowRtl,
  unstableScrollbarRtl,
  unstableSelectRtl,
  unstableSpaceRtl,
  unstableStatisticRtl,
  unstableStepsRtl,
  unstableTableRtl,
  unstableTagRtl,
  unstableThingRtl,
  unstableTreeRtl,
  unstableTreeSelectRtl,
  unstableUploadsRtl,
  ]
  : undefined))

useHead({
  htmlAttrs: {
    lang: () => locale.value,
    dir: () => direction.value,
  },
})
</script>

<template>
  <naive-config>
    <!-- Only the base theme lives here; the overrides come from `naiveui.themeConfig`. -->
    <n-config-provider :theme="isDark ? darkTheme : null" :locale="naiveLocale" :rtl="rtlStyles" inline-theme-disabled>
      <n-dialog-provider>
        <n-message-provider :max="3">
          <NuxtLayout>
            <NuxtPage />
          </NuxtLayout>
        </n-message-provider>
      </n-dialog-provider>
    </n-config-provider>
  </naive-config>
</template>
