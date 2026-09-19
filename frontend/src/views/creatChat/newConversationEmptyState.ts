import { defineComponent, h } from 'vue'

/**
 * New-conversation shell shared with an ongoing chat.
 *
 * Suggested questions and the composer sit together at the bottom. The area
 * above stays empty until a message exists. The campus slogan stays in the
 * locale file and is not rendered as a page title.
 */
export const NewConversationEmptyState = defineComponent({
  name: 'NewConversationEmptyState',
  setup(_, { slots }) {
    return () =>
      h('div', { class: 'dialogue-wrap', 'data-testid': 'new-conversation-empty' }, [
        h('div', { class: 'dialogue-answers' }, [slots.questions?.(), slots.composer?.()]),
      ])
  },
})
