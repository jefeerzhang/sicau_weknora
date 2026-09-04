import { defineComponent, h } from 'vue'
import { useI18n } from 'vue-i18n'

/**
 * New-conversation empty-state shell (#20).
 *
 * Keeps the visible title, suggested-questions region, and composer seam as
 * one public surface so tests can render localized output without mounting
 * the full creatChat.vue dependency graph (stores, router, InputField).
 */
export const NewConversationEmptyState = defineComponent({
  name: 'NewConversationEmptyState',
  setup(_, { slots }) {
    const { t } = useI18n()
    return () =>
      h('div', { class: 'dialogue-wrap', 'data-testid': 'new-conversation-empty' }, [
        h('div', { class: 'dialogue-answers' }, [
          h('div', { class: 'dialogue-title', style: { '--wails-draggable': 'drag' } }, [
            h('span', { style: { '--wails-draggable': 'drag' } }, t('createChat.title')),
          ]),
          slots.questions?.(),
          slots.composer?.(),
        ]),
      ])
  },
})
